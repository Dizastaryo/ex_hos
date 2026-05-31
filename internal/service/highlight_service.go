package service

import (
	"context"
	"fmt"

	"github.com/seeu/backend/internal/domain"
	"github.com/seeu/backend/internal/repository/postgres"
	redisRepo "github.com/seeu/backend/internal/repository/redis"
	"go.uber.org/zap"
)

type HighlightService struct {
	highlightRepo *postgres.HighlightRepository
	userRepo      *postgres.UserRepository
	followRepo    *postgres.FollowRepository
	cache         *redisRepo.Cache
	logger        *zap.Logger
}

func NewHighlightService(
	highlightRepo *postgres.HighlightRepository,
	userRepo *postgres.UserRepository,
	followRepo *postgres.FollowRepository,
	cache *redisRepo.Cache,
	logger *zap.Logger,
) *HighlightService {
	return &HighlightService{
		highlightRepo: highlightRepo,
		userRepo:      userRepo,
		followRepo:    followRepo,
		cache:         cache,
		logger:        logger,
	}
}

func (s *HighlightService) Create(ctx context.Context, userID string, req *domain.CreateHighlightRequest) (*domain.Highlight, error) {
	highlight := &domain.Highlight{
		UserID:   userID,
		Title:    req.Title,
		CoverURL: req.CoverURL,
	}

	if err := s.highlightRepo.Create(ctx, highlight); err != nil {
		return nil, fmt.Errorf("create highlight: %w", err)
	}

	if len(req.StoryIDs) > 0 {
		if err := s.highlightRepo.ReplaceStories(ctx, highlight.ID, req.StoryIDs); err != nil {
			s.logger.Warn("add stories to highlight", zap.Error(err))
		}
	}

	stories, err := s.highlightRepo.GetStories(ctx, highlight.ID)
	if err == nil {
		highlight.Stories = stories
	}

	return highlight, nil
}

func (s *HighlightService) GetByUsername(ctx context.Context, username, viewerID string) ([]*domain.Highlight, error) {
	// Приватный профиль: highlights видят только владелец и подтверждённые подписчики.
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

	highlights, err := s.highlightRepo.GetByUsername(ctx, username)
	if err != nil {
		return nil, err
	}

	for _, h := range highlights {
		stories, err := s.highlightRepo.GetStories(ctx, h.ID)
		if err == nil {
			h.Stories = stories
		}
	}

	return highlights, nil
}

func (s *HighlightService) Update(ctx context.Context, highlightID, userID string, req *domain.UpdateHighlightRequest) (*domain.Highlight, error) {
	highlight, err := s.highlightRepo.GetByID(ctx, highlightID)
	if err != nil {
		return nil, err
	}

	if highlight.UserID != userID {
		return nil, domain.ErrForbidden
	}

	if req.Title != "" {
		highlight.Title = req.Title
	}
	if req.CoverURL != "" {
		highlight.CoverURL = req.CoverURL
	}

	if err := s.highlightRepo.Update(ctx, highlight); err != nil {
		return nil, fmt.Errorf("update highlight: %w", err)
	}

	if len(req.StoryIDs) > 0 {
		if err := s.highlightRepo.ReplaceStories(ctx, highlight.ID, req.StoryIDs); err != nil {
			s.logger.Warn("replace highlight stories", zap.Error(err))
		}
	}

	stories, err := s.highlightRepo.GetStories(ctx, highlight.ID)
	if err == nil {
		highlight.Stories = stories
	}

	return highlight, nil
}

func (s *HighlightService) Delete(ctx context.Context, highlightID, userID string) error {
	highlight, err := s.highlightRepo.GetByID(ctx, highlightID)
	if err != nil {
		return err
	}

	if highlight.UserID != userID {
		return domain.ErrForbidden
	}

	return s.highlightRepo.Delete(ctx, highlightID, userID)
}
