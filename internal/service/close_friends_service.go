package service

import (
	"context"

	"github.com/seeu/backend/internal/domain"
	"github.com/seeu/backend/internal/repository/postgres"
	"go.uber.org/zap"
)

// CloseFriendsService — PROFILE-3. Owner кастомизирует свой CF-список,
// stories видят соответствующие подписчики.
type CloseFriendsService struct {
	repo     *postgres.CloseFriendsRepository
	userRepo *postgres.UserRepository
	logger   *zap.Logger
}

func NewCloseFriendsService(
	repo *postgres.CloseFriendsRepository,
	userRepo *postgres.UserRepository,
	logger *zap.Logger,
) *CloseFriendsService {
	return &CloseFriendsService{repo: repo, userRepo: userRepo, logger: logger}
}

func (s *CloseFriendsService) Add(ctx context.Context, ownerID, targetUsername string) error {
	target, err := s.userRepo.GetByUsername(ctx, targetUsername)
	if err != nil {
		return err
	}
	if target.ID == ownerID {
		return domain.ErrInvalidInput
	}
	return s.repo.Add(ctx, ownerID, target.ID)
}

func (s *CloseFriendsService) Remove(ctx context.Context, ownerID, targetUsername string) error {
	target, err := s.userRepo.GetByUsername(ctx, targetUsername)
	if err != nil {
		return err
	}
	return s.repo.Remove(ctx, ownerID, target.ID)
}

func (s *CloseFriendsService) List(ctx context.Context, ownerID string, limit, offset int) ([]*postgres.CloseFriendSummary, error) {
	return s.repo.List(ctx, ownerID, limit, offset)
}
