package service

import (
	"context"
	"fmt"
	"time"

	"github.com/seeu/backend/internal/domain"
	"github.com/seeu/backend/internal/repository/postgres"
	redisRepo "github.com/seeu/backend/internal/repository/redis"
	"github.com/seeu/backend/internal/ws"
	"github.com/seeu/backend/pkg/pagination"
	"go.uber.org/zap"
)

type UserService struct {
	userRepo    *postgres.UserRepository
	followRepo  *postgres.FollowRepository
	frRepo      *postgres.FollowRequestRepository
	scannerRepo *postgres.ScannerRepository
	statsRepo   *postgres.UserStatsRepository
	wsHub       *ws.Hub
	cache       *redisRepo.Cache
	logger      *zap.Logger
}

func NewUserService(
	userRepo *postgres.UserRepository,
	followRepo *postgres.FollowRepository,
	frRepo *postgres.FollowRequestRepository,
	scannerRepo *postgres.ScannerRepository,
	statsRepo *postgres.UserStatsRepository,
	wsHub *ws.Hub,
	cache *redisRepo.Cache,
	logger *zap.Logger,
) *UserService {
	return &UserService{
		userRepo:    userRepo,
		followRepo:  followRepo,
		frRepo:      frRepo,
		scannerRepo: scannerRepo,
		statsRepo:   statsRepo,
		wsHub:       wsHub,
		cache:       cache,
		logger:      logger,
	}
}

func (s *UserService) GetByID(ctx context.Context, id string) (*domain.User, error) {
	var user domain.User
	cacheKey := redisRepo.UserKey(id)

	if err := s.cache.Get(ctx, cacheKey, &user); err == nil {
		return &user, nil
	}

	u, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := s.cache.Set(ctx, cacheKey, u, cacheTTL); err != nil {
		s.logger.Warn("cache set user", zap.Error(err))
	}

	return u, nil
}

// BindDevice очищает device_public_id/private_id (используется при UnbindMyDevice).
// Для привязки по серийнику используй DeviceService.BindDeviceToUser.
func (s *UserService) BindDevice(ctx context.Context, userID, publicID string) error {
	user, _ := s.userRepo.GetByID(ctx, userID)
	if err := s.userRepo.SetDeviceIDs(ctx, userID, publicID, ""); err != nil {
		return err
	}
	if user != nil {
		s.invalidateUserCache(ctx, userID, user.Username)
	}
	return nil
}

// GetScanProfileByDeviceHash резолвит public_id_hex → scan-профиль.
// scan_enabled=false → ErrUserNotFound (сервер-сайд toggle).
// Возвращает только поля для анонимного scan-профиля.
func (s *UserService) GetScanProfileByDeviceHash(ctx context.Context, publicIDHex string) (*domain.ScanProfile, error) {
	user, err := s.userRepo.GetByDevicePublicID(ctx, publicIDHex)
	if err != nil {
		return nil, err
	}
	return &domain.ScanProfile{
		ScanAlias:     user.ScanAlias,
		ScanAvatarURL: user.ScanAvatarURL,
		DeviceHash:    user.DevicePublicID,
	}, nil
}

// GetByDevicePublicID returns the bare user record by BLE device public_id.
// Deprecated: используй GetScanProfileByDeviceHash для scanner UI.
func (s *UserService) GetByDevicePublicID(ctx context.Context, publicID string) (*domain.User, error) {
	return s.userRepo.GetByDevicePublicID(ctx, publicID)
}

// UpdateScanProfile обновляет scan-профиль (псевдоним, аватар, toggle видимости).
func (s *UserService) UpdateScanProfile(ctx context.Context, userID string, req *domain.UpdateScanProfileRequest) error {
	user, _ := s.userRepo.GetByID(ctx, userID)
	if err := s.scannerRepo.UpdateScanProfile(ctx, userID, req); err != nil {
		return err
	}
	if user != nil {
		s.invalidateUserCache(ctx, userID, user.Username)
	}
	return nil
}

// GetByDevicePrivateIDForViewer резолвит приватный BLE-id чипа.
// Новая логика: видит тот, кого ВЛАДЕЛЕЦ браслета добавил в свой private-whitelist.
// Раньше была проверка follow'ed viewer'а — заменена на явный whitelist от владельца.
//
// Если viewerID empty → ErrUnauthorized.
// Если viewerID не в whitelist владельца → ErrUserNotFound (не раскрываем факт существования).
func (s *UserService) GetByDevicePrivateIDForViewer(
	ctx context.Context, viewerID, privateID string,
) (*domain.User, error) {
	if viewerID == "" {
		return nil, domain.ErrUnauthorized
	}
	if privateID == "" {
		return nil, domain.ErrUserNotFound
	}
	// Используем scanner_repo: один SQL с проверкой whitelist внутри
	userID, err := s.scannerRepo.GetUserByPrivateDeviceHash(ctx, privateID, viewerID)
	if err != nil {
		if err == domain.ErrNotFound {
			return nil, domain.ErrUserNotFound
		}
		return nil, fmt.Errorf("resolve private device: %w", err)
	}
	return s.userRepo.GetByID(ctx, userID)
}

func (s *UserService) GetByUsername(ctx context.Context, username, viewerID string) (*domain.UserPublicProfile, error) {
	user, err := s.userRepo.GetByUsername(ctx, username)
	if err != nil {
		return nil, err
	}

	profile := toPublicProfile(user)

	// Social score: подгружаем total_likes из user_stats.
	if stats, err := s.statsRepo.GetByUserID(ctx, user.ID); err == nil {
		profile.TotalLikes = stats.TotalLikes
	}

	// PROFILE-6: privacy — если юзер скрыл last_seen, показываем только себе.
	// Для других зрителей: is_online=false и last_seen_at=zero.
	isSelf := viewerID == user.ID
	if !user.HideLastSeen || isSelf {
		if s.wsHub != nil {
			profile.IsOnline = s.wsHub.IsOnline(user.ID)
		}
	} else {
		profile.LastSeenAt = time.Time{}
	}

	if viewerID != "" && viewerID != user.ID {
		following, err := s.followRepo.IsFollowing(ctx, viewerID, user.ID)
		if err == nil {
			profile.IsFollowing = following
		}
		// Pending follow request виден только для приватного профиля,
		// чтобы Follow-кнопка показывала «Запрос отправлен». Для публичного
		// профиля поле не имеет смысла — там сразу подписываемся без request.
		if user.IsPrivate && !profile.IsFollowing {
			pending, err := s.frRepo.HasPending(ctx, viewerID, user.ID)
			if err == nil {
				profile.HasPendingFollowRequest = pending
			}
		}
	}

	return profile, nil
}

func (s *UserService) UpdateProfile(ctx context.Context, userID string, req *domain.UpdateProfileRequest) (*domain.User, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if req.FullName != "" {
		user.FullName = req.FullName
	}
	if req.Username != "" {
		user.Username = req.Username
	}
	if req.Bio != "" {
		user.Bio = req.Bio
	}
	user.Website = req.Website
	if req.Gender != "" {
		user.Gender = req.Gender
	}
	if req.IsPrivate != nil {
		user.IsPrivate = *req.IsPrivate
	}
	if req.HideLastSeen != nil {
		user.HideLastSeen = *req.HideLastSeen
	}
	if req.ChannelAbout != nil {
		user.ChannelAbout = *req.ChannelAbout
	}
	if req.ChannelBannerURL != nil {
		user.ChannelBannerURL = *req.ChannelBannerURL
	}
	if req.AvatarURL != "" {
		user.AvatarURL = req.AvatarURL
	}
	if req.DevicePublicID != "" {
		user.DevicePublicID = req.DevicePublicID
	}
	if req.DevicePrivateID != "" {
		user.DevicePrivateID = req.DevicePrivateID
	}

	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, fmt.Errorf("update user: %w", err)
	}

	s.invalidateUserCache(ctx, userID, user.Username)

	return user, nil
}

// DeleteAccount removes the user record. All owned content (posts, stories,
// comments, likes, follows, chats, files) is removed via ON DELETE CASCADE.
// Sessions are revoked by the caller (handler) so the in-flight access token
// becomes useless on next refresh.
func (s *UserService) DeleteAccount(ctx context.Context, userID string) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	if err := s.userRepo.DeleteByID(ctx, userID); err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	s.invalidateUserCache(ctx, userID, user.Username)
	return nil
}

func (s *UserService) GetFollowers(ctx context.Context, username, viewerID string, page, limit int) ([]*domain.UserShort, error) {
	user, err := s.userRepo.GetByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	if err := s.checkPrivacy(ctx, user, viewerID); err != nil {
		return nil, err
	}

	offset := pagination.Offset(page, limit)
	users, err := s.userRepo.GetFollowers(ctx, user.ID, limit, offset)
	if err != nil {
		return nil, err
	}

	return toUserShortList(users), nil
}

func (s *UserService) GetFollowing(ctx context.Context, username, viewerID string, page, limit int) ([]*domain.UserShort, error) {
	user, err := s.userRepo.GetByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	if err := s.checkPrivacy(ctx, user, viewerID); err != nil {
		return nil, err
	}

	offset := pagination.Offset(page, limit)
	users, err := s.userRepo.GetFollowing(ctx, user.ID, limit, offset)
	if err != nil {
		return nil, err
	}

	return toUserShortList(users), nil
}

func (s *UserService) GetMutuals(ctx context.Context, userID string) ([]*domain.UserShort, error) {
	users, err := s.followRepo.GetMutuals(ctx, userID)
	if err != nil {
		return nil, err
	}
	return toUserShortList(users), nil
}

// checkPrivacy — приватный профиль виден только владельцу и подписчикам.
// Используется для followers/following — Instagram скрывает оба списка
// у приватных юзеров от не-подписчиков.
func (s *UserService) checkPrivacy(ctx context.Context, user *domain.User, viewerID string) error {
	if !user.IsPrivate || viewerID == user.ID {
		return nil
	}
	if viewerID == "" {
		return domain.ErrPrivateAccount
	}
	isFollowing, err := s.followRepo.IsFollowing(ctx, viewerID, user.ID)
	if err != nil {
		return fmt.Errorf("check follower: %w", err)
	}
	if !isFollowing {
		return domain.ErrPrivateAccount
	}
	return nil
}

func (s *UserService) invalidateUserCache(ctx context.Context, userID, username string) {
	s.cache.InvalidateUser(ctx, userID, username)
}

// InvalidateCache — публичный метод для инвалидации кэша из хэндлера
// (например после привязки браслета через DeviceService).
func (s *UserService) InvalidateCache(ctx context.Context, userID string) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	s.invalidateUserCache(ctx, userID, user.Username)
	return nil
}

func toPublicProfile(u *domain.User) *domain.UserPublicProfile {
	return &domain.UserPublicProfile{
		ID:               u.ID,
		Username:         u.Username,
		FullName:         u.FullName,
		Bio:              u.Bio,
		AvatarURL:        u.AvatarURL,
		Website:          u.Website,
		Gender:           u.Gender,
		DevicePublicID:   u.DevicePublicID,
		IsPrivate:        u.IsPrivate,
		IsVerified:       u.IsVerified,
		PostsCount:       u.PostsCount,
		FollowersCount:   u.FollowersCount,
		FollowingCount:   u.FollowingCount,
		LastSeenAt:       u.LastSeenAt,
		ChannelAbout:     u.ChannelAbout,
		ChannelBannerURL: u.ChannelBannerURL,
		CreatedAt:        u.CreatedAt,
	}
}

func toUserShortList(users []*domain.User) []*domain.UserShort {
	result := make([]*domain.UserShort, len(users))
	for i, u := range users {
		result[i] = &domain.UserShort{
			ID:         u.ID,
			Username:   u.Username,
			FullName:   u.FullName,
			AvatarURL:  u.AvatarURL,
			IsVerified: u.IsVerified,
		}
	}
	return result
}
