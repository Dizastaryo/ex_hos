package service

import (
	"context"
	"fmt"

	"github.com/seeu/backend/internal/domain"
	"github.com/seeu/backend/internal/repository/postgres"
	redisRepo "github.com/seeu/backend/internal/repository/redis"
	"github.com/seeu/backend/internal/ws"
	"go.uber.org/zap"
)

type LikeService struct {
	likeRepo    *postgres.LikeRepository
	postRepo    *postgres.PostRepository
	commentRepo *postgres.CommentRepository
	storyRepo   *postgres.StoryRepository
	notifRepo   *postgres.NotificationRepository
	statsRepo   *postgres.UserStatsRepository
	cache       *redisRepo.Cache
	wsHub       *ws.Hub
	logger      *zap.Logger
}

func NewLikeService(
	likeRepo *postgres.LikeRepository,
	postRepo *postgres.PostRepository,
	commentRepo *postgres.CommentRepository,
	storyRepo *postgres.StoryRepository,
	notifRepo *postgres.NotificationRepository,
	statsRepo *postgres.UserStatsRepository,
	cache *redisRepo.Cache,
	wsHub *ws.Hub,
	logger *zap.Logger,
) *LikeService {
	return &LikeService{
		likeRepo:    likeRepo,
		postRepo:    postRepo,
		commentRepo: commentRepo,
		storyRepo:   storyRepo,
		notifRepo:   notifRepo,
		statsRepo:   statsRepo,
		cache:       cache,
		wsHub:       wsHub,
		logger:      logger,
	}
}

func (s *LikeService) LikePost(ctx context.Context, postID, userID string) error {
	post, err := s.postRepo.GetByID(ctx, postID)
	if err != nil {
		return err
	}

	// BACK-1: atomic like+counter в одной tx. Раньше если bump падал — like
	// был, а счётчик не обновлялся.
	if err := s.likeRepo.LikeEntityAtomic(ctx, userID, postID, domain.LikeEntityPost, "posts"); err != nil {
		return err
	}

	// Social score: увеличиваем post_likes владельца.
	if post.UserID != userID {
		if err := s.statsRepo.IncrementLikes(ctx, post.UserID, "post_likes"); err != nil {
			s.logger.Warn("increment post_likes", zap.Error(err))
		}
	}

	if post.UserID != userID {
		entityType := domain.LikeEntityPost
		n := &domain.Notification{
			UserID:     post.UserID,
			FromUserID: &userID,
			Type:       domain.NotificationTypeLike,
			EntityID:   &postID,
			EntityType: &entityType,
			Message:    "liked your post",
		}
		if err := s.notifRepo.Create(ctx, n); err != nil {
			s.logger.Warn("create like notification", zap.Error(err))
		} else {
			pushNotif(s.wsHub, n)
		}
	}

	return nil
}

func (s *LikeService) UnlikePost(ctx context.Context, postID, userID string) error {
	// BACK-1: atomic unlike.
	return s.likeRepo.UnlikeEntityAtomic(ctx, userID, postID, domain.LikeEntityPost, "posts")
}

func (s *LikeService) LikeComment(ctx context.Context, commentID, userID string) error {
	comment, err := s.commentRepo.GetByID(ctx, commentID)
	if err != nil {
		return err
	}

	// BACK-1: atomic like+counter.
	if err := s.likeRepo.LikeEntityAtomic(ctx, userID, commentID, domain.LikeEntityComment, "comments"); err != nil {
		return err
	}

	if comment.UserID != userID {
		entityType := domain.LikeEntityComment
		n := &domain.Notification{
			UserID:     comment.UserID,
			FromUserID: &userID,
			Type:       domain.NotificationTypeLike,
			EntityID:   &commentID,
			EntityType: &entityType,
			Message:    "liked your comment",
		}
		if err := s.notifRepo.Create(ctx, n); err != nil {
			s.logger.Warn("create comment like notification", zap.Error(err))
		} else {
			pushNotif(s.wsHub, n)
		}
	}

	return nil
}

func (s *LikeService) UnlikeComment(ctx context.Context, commentID, userID string) error {
	// BACK-1: atomic unlike.
	return s.likeRepo.UnlikeEntityAtomic(ctx, userID, commentID, domain.LikeEntityComment, "comments")
}

func (s *LikeService) LikeStory(ctx context.Context, storyID, userID string) error {
	story, err := s.storyRepo.GetByID(ctx, storyID)
	if err != nil {
		return err
	}

	like := &domain.Like{
		UserID:     userID,
		EntityID:   storyID,
		EntityType: domain.LikeEntityStory,
	}

	if err := s.likeRepo.Create(ctx, like); err != nil {
		return err
	}

	if err := s.storyRepo.IncrementLikesCount(ctx, storyID, 1); err != nil {
		s.logger.Warn("increment story likes count", zap.Error(err))
	}

	// Social score: увеличиваем story_likes владельца.
	if story.UserID != userID {
		if err := s.statsRepo.IncrementLikes(ctx, story.UserID, "story_likes"); err != nil {
			s.logger.Warn("increment story_likes", zap.Error(err))
		}
	}

	if story.UserID != userID {
		entityType := domain.LikeEntityStory
		n := &domain.Notification{
			UserID:     story.UserID,
			FromUserID: &userID,
			Type:       domain.NotificationTypeStoryLike,
			EntityID:   &storyID,
			EntityType: &entityType,
			Message:    "liked your story",
		}
		if err := s.notifRepo.Create(ctx, n); err != nil {
			s.logger.Warn("create story like notification", zap.Error(err))
		} else {
			pushNotif(s.wsHub, n)
		}
	}

	return nil
}

func (s *LikeService) UnlikeStory(ctx context.Context, storyID, userID string) error {
	if err := s.likeRepo.Delete(ctx, userID, storyID, domain.LikeEntityStory); err != nil {
		return err
	}

	if err := s.storyRepo.IncrementLikesCount(ctx, storyID, -1); err != nil {
		s.logger.Warn("decrement story likes count", zap.Error(err))
	}

	return nil
}

func (s *LikeService) SavePost(ctx context.Context, postID, userID string) error {
	_, err := s.postRepo.GetByID(ctx, postID)
	if err != nil {
		return err
	}

	inserted, err := s.postRepo.SavePost(ctx, userID, postID)
	if err != nil {
		return fmt.Errorf("save post: %w", err)
	}

	if inserted {
		if err := s.postRepo.IncrementSavesCount(ctx, postID, 1); err != nil {
			s.logger.Warn("increment saves count", zap.Error(err))
		}
	}

	return nil
}

func (s *LikeService) UnsavePost(ctx context.Context, postID, userID string) error {
	if err := s.postRepo.UnsavePost(ctx, userID, postID); err != nil {
		return err
	}

	if err := s.postRepo.IncrementSavesCount(ctx, postID, -1); err != nil {
		s.logger.Warn("decrement saves count", zap.Error(err))
	}

	return nil
}
