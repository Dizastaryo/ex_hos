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

type CommentService struct {
	commentRepo *postgres.CommentRepository
	postRepo    *postgres.PostRepository
	notifRepo   *postgres.NotificationRepository
	cache       *redisRepo.Cache
	wsHub       *ws.Hub
	logger      *zap.Logger
}

func NewCommentService(
	commentRepo *postgres.CommentRepository,
	postRepo *postgres.PostRepository,
	notifRepo *postgres.NotificationRepository,
	cache *redisRepo.Cache,
	wsHub *ws.Hub,
	logger *zap.Logger,
) *CommentService {
	return &CommentService{
		commentRepo: commentRepo,
		postRepo:    postRepo,
		notifRepo:   notifRepo,
		cache:       cache,
		wsHub:       wsHub,
		logger:      logger,
	}
}

func (s *CommentService) Create(ctx context.Context, postID, userID string, req *domain.CreateCommentRequest) (*domain.Comment, error) {
	post, err := s.postRepo.GetByID(ctx, postID)
	if err != nil {
		return nil, err
	}

	comment := &domain.Comment{
		PostID:   postID,
		UserID:   userID,
		ParentID: req.ParentID,
		Text:     req.Text,
	}

	if err := s.commentRepo.Create(ctx, comment); err != nil {
		return nil, fmt.Errorf("create comment: %w", err)
	}

	if err := s.postRepo.IncrementCommentsCount(ctx, postID, 1); err != nil {
		s.logger.Warn("increment comments count", zap.Error(err))
	}

	// Create notification for post owner (not if commenting on own post)
	if post.UserID != userID {
		entityType := domain.LikeEntityPost
		commentID := comment.ID
		n := &domain.Notification{
			UserID:     post.UserID,
			FromUserID: &userID,
			Type:       domain.NotificationTypeComment,
			EntityID:   &postID,
			EntityType: &entityType,
			CommentID:  &commentID,
			Message:    fmt.Sprintf("commented on your post: %s", truncate(req.Text, 50)),
		}
		if err := s.notifRepo.Create(ctx, n); err != nil {
			s.logger.Warn("create comment notification", zap.Error(err))
		} else {
			pushNotif(s.wsHub, n)
		}
	}

	return comment, nil
}

func (s *CommentService) Delete(ctx context.Context, commentID, userID string) error {
	comment, err := s.commentRepo.GetByID(ctx, commentID)
	if err != nil {
		return err
	}

	if comment.UserID != userID {
		// Also allow post owner to delete
		post, err := s.postRepo.GetByID(ctx, comment.PostID)
		if err != nil || post.UserID != userID {
			return domain.ErrForbidden
		}
	}

	if err := s.commentRepo.Delete(ctx, commentID, userID); err != nil {
		return err
	}

	if err := s.postRepo.IncrementCommentsCount(ctx, comment.PostID, -1); err != nil {
		s.logger.Warn("decrement comments count", zap.Error(err))
	}

	return nil
}

func (s *CommentService) GetByPostID(ctx context.Context, postID, viewerID string, page, limit int) ([]*domain.Comment, pagination.Meta, error) {
	_, err := s.postRepo.GetByID(ctx, postID)
	if err != nil {
		return nil, pagination.Meta{}, err
	}

	offset := pagination.Offset(page, limit)
	comments, err := s.commentRepo.GetByPostID(ctx, postID, viewerID, limit+1, offset)
	if err != nil {
		return nil, pagination.Meta{}, fmt.Errorf("get comments: %w", err)
	}

	hasNext := len(comments) > limit
	if hasNext {
		comments = comments[:limit]
	}

	if viewerID != "" {
		for _, c := range comments {
			liked, err := s.commentRepo.IsLikedByUser(ctx, c.ID, viewerID)
			if err == nil {
				c.IsLiked = liked
			}
		}
	}

	meta := pagination.Meta{
		Page:        page,
		Limit:       limit,
		HasNextPage: hasNext,
	}

	return comments, meta, nil
}

func (s *CommentService) GetReplies(ctx context.Context, parentID, viewerID string, page, limit int) ([]*domain.Comment, pagination.Meta, error) {
	offset := pagination.Offset(page, limit)
	replies, err := s.commentRepo.GetReplies(ctx, parentID, limit+1, offset)
	if err != nil {
		return nil, pagination.Meta{}, fmt.Errorf("get replies: %w", err)
	}

	hasNext := len(replies) > limit
	if hasNext {
		replies = replies[:limit]
	}

	meta := pagination.Meta{
		Page:        page,
		Limit:       limit,
		HasNextPage: hasNext,
	}

	return replies, meta, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
