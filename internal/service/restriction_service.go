package service

import (
	"context"

	"github.com/seeu/backend/internal/domain"
	"github.com/seeu/backend/internal/repository/postgres"
	"go.uber.org/zap"
)

// RestrictionService — PROFILE-4. Restrict (мягче чем block): ограниченный
// юзер может коммент'ить, но его комменты видны только ему + автору поста.
// Не удаляет follow-связь, не блокирует messages — только comment visibility.
type RestrictionService struct {
	repo     *postgres.RestrictionRepository
	userRepo *postgres.UserRepository
	logger   *zap.Logger
}

func NewRestrictionService(
	repo *postgres.RestrictionRepository,
	userRepo *postgres.UserRepository,
	logger *zap.Logger,
) *RestrictionService {
	return &RestrictionService{repo: repo, userRepo: userRepo, logger: logger}
}

func (s *RestrictionService) Restrict(ctx context.Context, userID, targetUsername string) error {
	target, err := s.userRepo.GetByUsername(ctx, targetUsername)
	if err != nil {
		return err
	}
	if target.ID == userID {
		return domain.ErrInvalidInput
	}
	return s.repo.Create(ctx, userID, target.ID)
}

func (s *RestrictionService) Unrestrict(ctx context.Context, userID, targetUsername string) error {
	target, err := s.userRepo.GetByUsername(ctx, targetUsername)
	if err != nil {
		return err
	}
	return s.repo.Delete(ctx, userID, target.ID)
}

func (s *RestrictionService) IsRestricted(ctx context.Context, userID, targetID string) (bool, error) {
	return s.repo.IsRestricted(ctx, userID, targetID)
}

func (s *RestrictionService) List(ctx context.Context, userID string, limit, offset int) ([]*postgres.RestrictedUserSummary, error) {
	return s.repo.List(ctx, userID, limit, offset)
}
