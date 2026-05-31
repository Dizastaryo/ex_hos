package service

import (
	"context"

	"github.com/seeu/backend/internal/domain"
	"github.com/seeu/backend/internal/repository/postgres"
	redisRepo "github.com/seeu/backend/internal/repository/redis"
	"github.com/seeu/backend/internal/ws"
	"go.uber.org/zap"
)

type FollowService struct {
	followRepo    *postgres.FollowRepository
	frRepo        *postgres.FollowRequestRepository
	userRepo      *postgres.UserRepository
	notifRepo     *postgres.NotificationRepository
	blockRepo     *postgres.BlockRepository
	cache         *redisRepo.Cache
	wsHub         *ws.Hub
	logger        *zap.Logger
}

func NewFollowService(
	followRepo *postgres.FollowRepository,
	frRepo *postgres.FollowRequestRepository,
	userRepo *postgres.UserRepository,
	notifRepo *postgres.NotificationRepository,
	blockRepo *postgres.BlockRepository,
	cache *redisRepo.Cache,
	wsHub *ws.Hub,
	logger *zap.Logger,
) *FollowService {
	return &FollowService{
		followRepo: followRepo,
		frRepo:     frRepo,
		userRepo:   userRepo,
		notifRepo:  notifRepo,
		blockRepo:  blockRepo,
		cache:      cache,
		wsHub:      wsHub,
		logger:     logger,
	}
}

// FollowResult tells the caller what actually happened: a follow was created
// (target is public) or a follow request was queued (target is private).
type FollowResult struct {
	Status string `json:"status"` // "following" | "requested"
}

func (s *FollowService) Follow(ctx context.Context, followerID, username string) (FollowResult, error) {
	if err := s.validateFollow(ctx, followerID, username); err != nil {
		return FollowResult{}, err
	}

	target, err := s.userRepo.GetByUsername(ctx, username)
	if err != nil {
		return FollowResult{}, err
	}

	if followerID == target.ID {
		return FollowResult{}, domain.ErrSelfFollow
	}

	// Refuse follow if either side has blocked the other.
	if blocked, err := s.blockRepo.IsEitherBlocked(ctx, followerID, target.ID); err != nil {
		return FollowResult{}, err
	} else if blocked {
		return FollowResult{}, domain.ErrForbidden
	}

	// Private account: queue a request instead. Existing follow row (e.g. user
	// went private after we already followed) is left untouched.
	if target.IsPrivate {
		alreadyFollowing, _ := s.followRepo.IsFollowing(ctx, followerID, target.ID)
		if alreadyFollowing {
			return FollowResult{Status: "following"}, nil
		}
		if _, err := s.frRepo.Create(ctx, followerID, target.ID); err != nil {
			if err == domain.ErrAlreadyExists {
				return FollowResult{Status: "requested"}, nil
			}
			return FollowResult{}, err
		}
		// Notify target so they see the request without polling.
		n := &domain.Notification{
			UserID:     target.ID,
			FromUserID: &followerID,
			Type:       domain.NotificationTypeFollow,
			Message:    "requested to follow you",
		}
		if err := s.notifRepo.Create(ctx, n); err == nil {
			pushNotif(s.wsHub, n)
		}
		return FollowResult{Status: "requested"}, nil
	}

	// BACK-1: атомарный flow — INSERT follow + bump обоих counters в одной
	// транзакции. Раньше: 3 отдельные операции, если 2-я падала — счётчики
	// рассинхрон. Notification создаётся отдельно (best-effort) — она не
	// критична к транзакционной consistency.
	if err := s.followRepo.CreateAtomic(ctx, followerID, target.ID); err != nil {
		return FollowResult{}, err
	}

	n := &domain.Notification{
		UserID:     target.ID,
		FromUserID: &followerID,
		Type:       domain.NotificationTypeFollow,
		Message:    "started following you",
	}
	if err := s.notifRepo.Create(ctx, n); err != nil {
		s.logger.Warn("create follow notification", zap.Error(err))
	} else {
		pushNotif(s.wsHub, n)
	}

	s.invalidateUserCache(ctx, followerID, target.ID)

	return FollowResult{Status: "following"}, nil
}

func (s *FollowService) Unfollow(ctx context.Context, followerID, username string) error {
	target, err := s.userRepo.GetByUsername(ctx, username)
	if err != nil {
		return err
	}

	// If the relationship was a pending request (private target), the unfollow
	// gesture cancels it. We try the request delete first; if no rows changed,
	// fall back to deleting the actual follow row.
	if pending, _ := s.frRepo.HasPending(ctx, followerID, target.ID); pending {
		_ = s.frRepo.DeletePair(ctx, followerID, target.ID)
		s.invalidateUserCache(ctx, followerID, target.ID)
		return nil
	}

	// BACK-1: атомарный unfollow — DELETE + decrement обоих counters в tx.
	if err := s.followRepo.DeleteAtomic(ctx, followerID, target.ID); err != nil {
		return err
	}

	s.invalidateUserCache(ctx, followerID, target.ID)

	return nil
}

// ListPendingRequests returns follow requests addressed to the current user.
func (s *FollowService) ListPendingRequests(ctx context.Context, userID string, limit, offset int) ([]*postgres.FollowRequest, error) {
	return s.frRepo.ListForTarget(ctx, userID, limit, offset)
}

// HasPendingRequest tells whether `requesterID` is awaiting approval from `targetID`.
func (s *FollowService) HasPendingRequest(ctx context.Context, requesterID, targetID string) (bool, error) {
	return s.frRepo.HasPending(ctx, requesterID, targetID)
}

// AcceptRequest creates the actual follow row + bumps counters + notifies the
// requester. Only the request's target may approve.
func (s *FollowService) AcceptRequest(ctx context.Context, requestID, callerID string) error {
	req, err := s.frRepo.GetByID(ctx, requestID)
	if err != nil {
		return err
	}
	if req.TargetID != callerID {
		return domain.ErrForbidden
	}
	if err := s.followRepo.Create(ctx, req.RequesterID, req.TargetID); err != nil &&
		err != domain.ErrAlreadyFollowing {
		return err
	}
	_ = s.userRepo.IncrementFollowersCount(ctx, req.TargetID, 1)
	_ = s.userRepo.IncrementFollowingCount(ctx, req.RequesterID, 1)
	if err := s.frRepo.Delete(ctx, req.ID); err != nil {
		return err
	}

	// Notify requester so they see «X принял заявку» in realtime.
	n := &domain.Notification{
		UserID:     req.RequesterID,
		FromUserID: &req.TargetID,
		Type:       domain.NotificationTypeFollow,
		Message:    "accepted your follow request",
	}
	if err := s.notifRepo.Create(ctx, n); err == nil {
		pushNotif(s.wsHub, n)
	}

	s.invalidateUserCache(ctx, req.RequesterID, req.TargetID)
	return nil
}

// DeclineRequest just deletes the request — no notification (silent reject).
func (s *FollowService) DeclineRequest(ctx context.Context, requestID, callerID string) error {
	req, err := s.frRepo.GetByID(ctx, requestID)
	if err != nil {
		return err
	}
	if req.TargetID != callerID {
		return domain.ErrForbidden
	}
	return s.frRepo.Delete(ctx, req.ID)
}

func (s *FollowService) IsFollowing(ctx context.Context, followerID, followingID string) (bool, error) {
	return s.followRepo.IsFollowing(ctx, followerID, followingID)
}

func (s *FollowService) validateFollow(ctx context.Context, followerID, username string) error {
	follower, err := s.userRepo.GetByID(ctx, followerID)
	if err != nil {
		return err
	}
	_ = follower

	target, err := s.userRepo.GetByUsername(ctx, username)
	if err != nil {
		return err
	}

	if followerID == target.ID {
		return domain.ErrSelfFollow
	}

	return nil
}

// invalidateUserCache drops the cached profile for both ends of a follow
// edge AND every feed page across all users (a graph change can flip what
// posts each follower sees). Bug fix 2026-05-10: previously this passed
// the literal string `feed:<follower>:*` to `cache.Delete`, which doesn't
// expand wildcards — so feed pages stayed stale for the entire TTL.
func (s *FollowService) invalidateUserCache(ctx context.Context, followerID, followingID string) {
	s.cache.InvalidateUser(ctx, followerID, "")
	s.cache.InvalidateUser(ctx, followingID, "")
	s.cache.InvalidateAllFeeds(ctx)
}
