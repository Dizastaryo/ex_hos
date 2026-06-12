package service

import (
	"context"

	"github.com/seeu/backend/internal/domain"
	"github.com/seeu/backend/internal/repository/postgres"
	"go.uber.org/zap"
)

type VideoService struct {
	videoRepo *postgres.VideoRepository
	statsRepo *postgres.UserStatsRepository
	logger    *zap.Logger
}

func NewVideoService(videoRepo *postgres.VideoRepository, statsRepo *postgres.UserStatsRepository, logger *zap.Logger) *VideoService {
	return &VideoService{videoRepo: videoRepo, statsRepo: statsRepo, logger: logger}
}

func (s *VideoService) CreateVideo(ctx context.Context, userID string, req *domain.CreateVideoRequest) (*domain.Video, error) {
	video := &domain.Video{
		UserID:          userID,
		Title:           req.Title,
		Description:     req.Description,
		VideoURL:        req.VideoURL,
		ThumbnailURL:    req.ThumbnailURL,
		DurationSeconds: req.DurationSeconds,
		CategoryID:      req.CategoryID,
		Resolution:      req.Resolution,
	}
	if err := s.videoRepo.CreateVideo(ctx, video); err != nil {
		return nil, err
	}
	return s.videoRepo.GetVideoByID(ctx, video.ID)
}

func (s *VideoService) GetVideo(ctx context.Context, id, viewerID string) (*domain.Video, error) {
	video, err := s.videoRepo.GetVideoByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if viewerID != "" {
		video.IsLiked, _ = s.videoRepo.IsVideoLiked(ctx, id, viewerID)
	}
	return video, nil
}

func (s *VideoService) ListVideos(ctx context.Context, categoryID string, limit, offset int) ([]*domain.Video, int, error) {
	return s.videoRepo.ListVideos(ctx, categoryID, limit, offset)
}

func (s *VideoService) GetFeatured(ctx context.Context) (*domain.Video, error) {
	return s.videoRepo.GetFeatured(ctx)
}

func (s *VideoService) GetUserVideos(ctx context.Context, ownerID, viewerID string, limit, offset int) ([]*domain.Video, int, error) {
	if err := s.videoRepo.CheckUserVisibility(ctx, ownerID, viewerID); err != nil {
		return nil, 0, err
	}
	return s.videoRepo.GetUserVideos(ctx, ownerID, limit, offset)
}

func (s *VideoService) DeleteVideo(ctx context.Context, id, userID string) error {
	return s.videoRepo.DeleteVideo(ctx, id, userID)
}

func (s *VideoService) ViewVideo(ctx context.Context, videoID, userID string) error {
	return s.videoRepo.IncrementViews(ctx, videoID, userID)
}

func (s *VideoService) LikeVideo(ctx context.Context, videoID, userID string) error {
	video, err := s.videoRepo.GetVideoByID(ctx, videoID)
	if err != nil {
		return err
	}
	if err := s.videoRepo.LikeVideo(ctx, videoID, userID); err != nil {
		return err
	}
	// Social score: не считаем лайки себе
	if video.UserID != userID {
		if err := s.statsRepo.IncrementLikes(ctx, video.UserID, "video_likes"); err != nil {
			s.logger.Warn("increment video_likes", zap.Error(err))
		}
	}
	return nil
}

func (s *VideoService) UnlikeVideo(ctx context.Context, videoID, userID string) error {
	return s.videoRepo.UnlikeVideo(ctx, videoID, userID)
}

func (s *VideoService) GetCategories(ctx context.Context) ([]*domain.VideoCategory, error) {
	return s.videoRepo.GetCategories(ctx)
}

// Reels-related methods removed — see migration 23 (reels merged into posts).
