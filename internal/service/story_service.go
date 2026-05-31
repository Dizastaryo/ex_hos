package service

import (
	"context"
	"fmt"

	"github.com/seeu/backend/internal/domain"
	"github.com/seeu/backend/internal/repository/postgres"
	redisRepo "github.com/seeu/backend/internal/repository/redis"
	"github.com/seeu/backend/internal/ws"
	"github.com/seeu/backend/pkg/pagination"
	"go.uber.org/zap"
)

type StoryService struct {
	storyRepo    *postgres.StoryRepository
	userRepo     *postgres.UserRepository
	followRepo   *postgres.FollowRepository
	notifRepo    *postgres.NotificationRepository
	cache        *redisRepo.Cache
	wsHub        *ws.Hub
	mediaService *MediaService
	logger       *zap.Logger
}

func NewStoryService(
	storyRepo *postgres.StoryRepository,
	userRepo *postgres.UserRepository,
	followRepo *postgres.FollowRepository,
	notifRepo *postgres.NotificationRepository,
	cache *redisRepo.Cache,
	wsHub *ws.Hub,
	mediaService *MediaService,
	logger *zap.Logger,
) *StoryService {
	return &StoryService{
		storyRepo:    storyRepo,
		userRepo:     userRepo,
		followRepo:   followRepo,
		notifRepo:    notifRepo,
		cache:        cache,
		wsHub:        wsHub,
		mediaService: mediaService,
		logger:       logger,
	}
}

func (s *StoryService) Create(ctx context.Context, userID string, req *domain.CreateStoryRequest) (*domain.Story, error) {
	duration := req.Duration
	if duration == 0 {
		duration = 5
	}

	// STORY-1: text-сторис не имеет media_url — проверяем только что текст
	// не пустой. Для image/video media_url обязателен.
	switch req.MediaType {
	case "text":
		if req.TextOverlay == "" {
			return nil, fmt.Errorf("text story requires text_overlay: %w", domain.ErrInvalidInput)
		}
	case "image", "video":
		if req.MediaURL == "" {
			return nil, fmt.Errorf("%s story requires media_url: %w", req.MediaType, domain.ErrInvalidInput)
		}
	}

	// STORY-3: poll-overlay валидируем — вопрос + оба варианта non-empty.
	if req.Poll != nil {
		if req.Poll.Question == "" || req.Poll.OptionA == "" || req.Poll.OptionB == "" {
			return nil, fmt.Errorf("poll requires question and two options: %w", domain.ErrInvalidInput)
		}
	}

	story := &domain.Story{
		UserID:             userID,
		MediaURL:           req.MediaURL,
		MediaType:          req.MediaType,
		Duration:           duration,
		TextOverlay:        req.TextOverlay,
		AudioTrackID:       req.AudioTrackID,
		SharedPostID:       req.SharedPostID,
		AudioStartSeconds:  req.AudioStartSeconds,
		BgColor:            req.BgColor,
		Poll:               req.Poll,
		IsCloseFriendsOnly: req.IsCloseFriendsOnly,
	}

	if err := s.storyRepo.Create(ctx, story); err != nil {
		return nil, fmt.Errorf("create story: %w", err)
	}

	s.invalidateStoryFeedCache(ctx, userID)

	user, err := s.userRepo.GetByID(ctx, userID)
	if err == nil {
		story.User = &domain.UserShort{
			ID:         user.ID,
			Username:   user.Username,
			FullName:   user.FullName,
			AvatarURL:  user.AvatarURL,
			IsVerified: user.IsVerified,
		}
	}

	// FEED-6 realtime: уведомляем followers что появилась new story. Они
	// обновят свой stories-row без manual refresh. Best-effort — если
	// followers offline, payload теряется (story всё равно появится при
	// next storyFeed pull).
	if s.wsHub != nil {
		if followers, ferr := s.followRepo.GetFollowerIDs(ctx, userID); ferr == nil {
			payload := map[string]any{
				"story_id": story.ID,
				"user_id":  userID,
			}
			for _, fid := range followers {
				s.wsHub.SendToUser(fid, "story.created", payload)
			}
		}
	}

	return story, nil
}

// GetAllByUsername возвращает все сторис пользователя включая истёкшие.
// Доступно только самому владельцу (для highlights picker).
func (s *StoryService) GetAllByUsername(ctx context.Context, username, viewerID string) ([]*domain.Story, error) {
	owner, err := s.userRepo.GetByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	if owner.ID != viewerID {
		return nil, domain.ErrForbidden
	}
	return s.storyRepo.GetAllByUsername(ctx, username)
}

func (s *StoryService) GetByUsername(ctx context.Context, username, viewerID string) ([]*domain.Story, error) {
	// Приватный профиль: stories видят только владелец и подтверждённые подписчики.
	owner, err := s.userRepo.GetByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	if owner.IsPrivate && viewerID != owner.ID {
		if viewerID == "" {
			return nil, domain.ErrPrivateAccount
		}
		isFollowing, err := s.followRepo.IsFollowing(ctx, viewerID, owner.ID)
		if err != nil {
			return nil, fmt.Errorf("check follower: %w", err)
		}
		if !isFollowing {
			return nil, domain.ErrPrivateAccount
		}
	}

	stories, err := s.storyRepo.GetByUsername(ctx, username, viewerID)
	if err != nil {
		return nil, err
	}

	if viewerID != "" {
		for _, story := range stories {
			viewed, err := s.storyRepo.IsViewedByUser(ctx, story.ID, viewerID)
			if err == nil {
				story.IsViewed = viewed
			}
		}
	}

	if err := s.storyRepo.AttachReactions(ctx, stories, viewerID); err != nil {
		s.logger.Warn("attach story reactions", zap.Error(err))
	}
	if err := s.storyRepo.AttachPollVotes(ctx, stories, viewerID); err != nil {
		s.logger.Warn("attach poll votes", zap.Error(err))
	}

	return stories, nil
}

func (s *StoryService) GetFeed(ctx context.Context, userID string) ([]*domain.StoryFeedGroup, error) {
	stories, err := s.storyRepo.GetFeed(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get story feed: %w", err)
	}

	// Group by user
	groupMap := make(map[string]*domain.StoryFeedGroup)
	var order []string

	for _, story := range stories {
		viewed, err := s.storyRepo.IsViewedByUser(ctx, story.ID, userID)
		if err == nil {
			story.IsViewed = viewed
		}

		if _, ok := groupMap[story.UserID]; !ok {
			groupMap[story.UserID] = &domain.StoryFeedGroup{
				User:    story.User,
				Stories: []*domain.Story{},
				HasSeen: true,
			}
			order = append(order, story.UserID)
		}

		groupMap[story.UserID].Stories = append(groupMap[story.UserID].Stories, story)
		if !story.IsViewed {
			groupMap[story.UserID].HasSeen = false
		}
	}

	if err := s.storyRepo.AttachReactions(ctx, stories, userID); err != nil {
		s.logger.Warn("attach story reactions (feed)", zap.Error(err))
	}
	if err := s.storyRepo.AttachPollVotes(ctx, stories, userID); err != nil {
		s.logger.Warn("attach poll votes (feed)", zap.Error(err))
	}

	groups := make([]*domain.StoryFeedGroup, 0, len(order))
	// Unseen first
	for _, uid := range order {
		if !groupMap[uid].HasSeen {
			groups = append(groups, groupMap[uid])
		}
	}
	for _, uid := range order {
		if groupMap[uid].HasSeen {
			groups = append(groups, groupMap[uid])
		}
	}

	return groups, nil
}

func (s *StoryService) Delete(ctx context.Context, storyID, userID string) error {
	// Capture media URL before delete so we can release dedup ref afterwards.
	var mediaURL string
	if story, err := s.storyRepo.GetByID(ctx, storyID); err == nil && story != nil {
		mediaURL = story.MediaURL
	}

	if err := s.storyRepo.Delete(ctx, storyID, userID); err != nil {
		return err
	}

	s.invalidateStoryFeedCache(ctx, userID)

	if s.mediaService != nil && mediaURL != "" {
		s.mediaService.Release(ctx, []string{mediaURL})
	}
	return nil
}

func (s *StoryService) AddView(ctx context.Context, storyID, viewerID string) error {
	story, err := s.storyRepo.GetByID(ctx, storyID)
	if err != nil {
		return err
	}

	if story.UserID == viewerID {
		return nil
	}

	if err := s.storyRepo.AddView(ctx, storyID, viewerID); err != nil {
		return err
	}

	// Re-fetch to get the post-increment views_count, then unicast to the
	// author so their open viewer can refresh the badge in realtime. Same
	// owner-private fan-out pattern as story.reaction events.
	if updated, err := s.storyRepo.GetByID(ctx, storyID); err == nil {
		pushStoryViewAdded(s.wsHub, story.UserID, storyID, updated.ViewsCount)
	}
	return nil
}

func (s *StoryService) GetViewers(ctx context.Context, storyID, ownerID string, page, limit int) ([]*domain.StoryViewer, pagination.Meta, error) {
	story, err := s.storyRepo.GetByID(ctx, storyID)
	if err != nil {
		return nil, pagination.Meta{}, err
	}

	if story.UserID != ownerID {
		return nil, pagination.Meta{}, domain.ErrForbidden
	}

	offset := pagination.Offset(page, limit)
	viewers, err := s.storyRepo.GetViewers(ctx, storyID, limit+1, offset)
	if err != nil {
		return nil, pagination.Meta{}, fmt.Errorf("get story viewers: %w", err)
	}

	hasNext := len(viewers) > limit
	if hasNext {
		viewers = viewers[:limit]
	}

	meta := pagination.Meta{
		Page:        page,
		Limit:       limit,
		HasNextPage: hasNext,
	}

	return viewers, meta, nil
}

func (s *StoryService) GetByID(ctx context.Context, storyID string) (*domain.Story, error) {
	return s.storyRepo.GetByID(ctx, storyID)
}

// SetReaction upserts a viewer's emoji reaction on a story. Returns the new
// aggregate counts. Pushes a unicast `story.reaction` event to the author —
// stories are author-private from an analytics perspective.
func (s *StoryService) SetReaction(ctx context.Context, storyID, userID, emoji string) (map[string]int, error) {
	story, err := s.storyRepo.GetByID(ctx, storyID)
	if err != nil {
		return nil, err
	}
	if err := s.storyRepo.SetReaction(ctx, storyID, userID, emoji); err != nil {
		return nil, err
	}
	counts, err := s.storyRepo.CountReactions(ctx, storyID)
	if err != nil {
		counts = map[string]int{}
	}
	pushStoryReaction(s.wsHub, story.UserID, storyID, counts)
	return counts, nil
}

// RemoveReaction deletes a viewer's reaction on a story.
func (s *StoryService) RemoveReaction(ctx context.Context, storyID, userID string) (map[string]int, error) {
	story, err := s.storyRepo.GetByID(ctx, storyID)
	if err != nil {
		return nil, err
	}
	if err := s.storyRepo.RemoveReaction(ctx, storyID, userID); err != nil {
		return nil, err
	}
	counts, err := s.storyRepo.CountReactions(ctx, storyID)
	if err != nil {
		counts = map[string]int{}
	}
	pushStoryReaction(s.wsHub, story.UserID, storyID, counts)
	return counts, nil
}

// VotePoll (STORY-3) — записать голос viewer'а на poll-overlay стори.
// Возвращает обновлённый Poll с counts + myVote.
func (s *StoryService) VotePoll(
	ctx context.Context, storyID, viewerID string, optionIndex int,
) (*domain.StoryPoll, error) {
	story, err := s.storyRepo.GetByID(ctx, storyID)
	if err != nil {
		return nil, err
	}
	if story.Poll == nil {
		return nil, fmt.Errorf("story has no poll: %w", domain.ErrInvalidInput)
	}
	// Не дать автору голосовать на своём же poll'е — это аномалия.
	if story.UserID == viewerID {
		return nil, fmt.Errorf("cannot vote on own poll: %w", domain.ErrForbidden)
	}
	votesA, votesB, err := s.storyRepo.VotePoll(ctx, storyID, viewerID, optionIndex)
	if err != nil {
		return nil, err
	}
	story.Poll.VotesA = votesA
	story.Poll.VotesB = votesB
	story.Poll.MyVote = optionIndex
	return story.Poll, nil
}

func (s *StoryService) invalidateStoryFeedCache(ctx context.Context, userID string) {
	followerIDs, err := s.followRepo.GetFollowerIDs(ctx, userID)
	if err != nil {
		s.logger.Warn("get follower ids for cache invalidation", zap.Error(err))
		return
	}

	keys := []string{redisRepo.StoryFeedKey(userID)}
	for _, fid := range followerIDs {
		keys = append(keys, redisRepo.StoryFeedKey(fid))
	}

	if err := s.cache.Delete(ctx, keys...); err != nil {
		s.logger.Warn("invalidate story feed cache", zap.Error(err))
	}
}
