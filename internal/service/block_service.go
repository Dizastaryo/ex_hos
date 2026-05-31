package service

import (
	"context"
	"fmt"

	"github.com/seeu/backend/internal/domain"
	"github.com/seeu/backend/internal/repository/postgres"
	"go.uber.org/zap"
)

type BlockService struct {
	repo       *postgres.BlockRepository
	userRepo   *postgres.UserRepository
	followRepo *postgres.FollowRepository
	logger     *zap.Logger
}

func NewBlockService(
	repo *postgres.BlockRepository,
	userRepo *postgres.UserRepository,
	followRepo *postgres.FollowRepository,
	logger *zap.Logger,
) *BlockService {
	return &BlockService{repo: repo, userRepo: userRepo, followRepo: followRepo, logger: logger}
}

// Block marks `targetUsername` as blocked by `blockerID`. Side effects:
//   - any follow relationship between the two users is removed (both directions)
// We do NOT cascade-delete content (their old comments on each other's posts
// stay where they are) — just hide it from feeds/profile views.
func (s *BlockService) Block(ctx context.Context, blockerID, targetUsername string) error {
	target, err := s.userRepo.GetByUsername(ctx, targetUsername)
	if err != nil {
		return err
	}
	if target.ID == blockerID {
		return domain.ErrInvalidInput
	}
	if err := s.repo.Create(ctx, blockerID, target.ID); err != nil {
		return err
	}
	// Drop both follow edges and keep the denormalized counters in sync.
	// `Delete` returns ErrNotFollowing if there was no row — in that case we
	// must NOT decrement counters. We only adjust counters on a real delete.
	if err := s.followRepo.Delete(ctx, blockerID, target.ID); err == nil {
		if err := s.userRepo.IncrementFollowingCount(ctx, blockerID, -1); err != nil {
			s.logger.Warn("decrement following count on block", zap.Error(err))
		}
		if err := s.userRepo.IncrementFollowersCount(ctx, target.ID, -1); err != nil {
			s.logger.Warn("decrement followers count on block", zap.Error(err))
		}
	} else if err != domain.ErrNotFollowing {
		s.logger.Warn("drop follow on block", zap.Error(err))
	}
	if err := s.followRepo.Delete(ctx, target.ID, blockerID); err == nil {
		if err := s.userRepo.IncrementFollowingCount(ctx, target.ID, -1); err != nil {
			s.logger.Warn("decrement following count on block (reverse)", zap.Error(err))
		}
		if err := s.userRepo.IncrementFollowersCount(ctx, blockerID, -1); err != nil {
			s.logger.Warn("decrement followers count on block (reverse)", zap.Error(err))
		}
	} else if err != domain.ErrNotFollowing {
		s.logger.Warn("drop reverse follow on block", zap.Error(err))
	}
	return nil
}

func (s *BlockService) Unblock(ctx context.Context, blockerID, targetUsername string) error {
	target, err := s.userRepo.GetByUsername(ctx, targetUsername)
	if err != nil {
		return err
	}
	return s.repo.Delete(ctx, blockerID, target.ID)
}

func (s *BlockService) IsEitherBlocked(ctx context.Context, a, b string) (bool, error) {
	if a == b {
		return false, nil
	}
	return s.repo.IsEitherBlocked(ctx, a, b)
}

// GuardInteraction returns ErrForbidden if a and b have a block relationship in
// either direction. Convenience for handlers that want a single check.
func (s *BlockService) GuardInteraction(ctx context.Context, a, b string) error {
	blocked, err := s.IsEitherBlocked(ctx, a, b)
	if err != nil {
		return fmt.Errorf("guard interaction: %w", err)
	}
	if blocked {
		return domain.ErrForbidden
	}
	return nil
}

func (s *BlockService) ListBlocked(ctx context.Context, blockerID string, limit, offset int) ([]*postgres.BlockedUserSummary, error) {
	return s.repo.ListBlocked(ctx, blockerID, limit, offset)
}
