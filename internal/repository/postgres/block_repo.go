package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/seeu/backend/internal/domain"
)

type BlockRepository struct {
	db *DB
}

func NewBlockRepository(db *DB) *BlockRepository {
	return &BlockRepository{db: db}
}

// Create records a block from blocker to blocked. Idempotent — re-blocking the
// same user is a no-op.
func (r *BlockRepository) Create(ctx context.Context, blockerID, blockedID string) error {
	if blockerID == blockedID {
		return domain.ErrInvalidInput
	}
	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO user_blocks (blocker_id, blocked_id)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING`, blockerID, blockedID)
	if err != nil {
		return fmt.Errorf("create block: %w", err)
	}
	return nil
}

func (r *BlockRepository) Delete(ctx context.Context, blockerID, blockedID string) error {
	_, err := r.db.Pool.Exec(ctx,
		`DELETE FROM user_blocks WHERE blocker_id = $1 AND blocked_id = $2`,
		blockerID, blockedID)
	if err != nil {
		return fmt.Errorf("delete block: %w", err)
	}
	return nil
}

// IsEitherBlocked returns true if user a has blocked b OR vice versa.
// Used at every visibility/interaction gate (feed, profile, chat send, follow).
func (r *BlockRepository) IsEitherBlocked(ctx context.Context, a, b string) (bool, error) {
	var n int
	err := r.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM user_blocks
		WHERE (blocker_id = $1 AND blocked_id = $2)
		   OR (blocker_id = $2 AND blocked_id = $1)`, a, b).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("check block: %w", err)
	}
	return n > 0, nil
}

type BlockedUserSummary struct {
	UserID     string    `json:"user_id"`
	Username   string    `json:"username"`
	FullName   string    `json:"full_name"`
	AvatarURL  string    `json:"avatar_url"`
	BlockedAt  time.Time `json:"blocked_at"`
}

func (r *BlockRepository) ListBlocked(ctx context.Context, blockerID string, limit, offset int) ([]*BlockedUserSummary, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.Pool.Query(ctx, `
		SELECT u.id, u.username, u.full_name, u.avatar_url, b.created_at
		FROM user_blocks b
		JOIN users u ON u.id = b.blocked_id
		WHERE b.blocker_id = $1
		ORDER BY b.created_at DESC
		LIMIT $2 OFFSET $3`, blockerID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list blocked: %w", err)
	}
	defer rows.Close()

	var out []*BlockedUserSummary
	for rows.Next() {
		s := &BlockedUserSummary{}
		if err := rows.Scan(&s.UserID, &s.Username, &s.FullName, &s.AvatarURL, &s.BlockedAt); err != nil {
			return nil, fmt.Errorf("list blocked scan: %w", err)
		}
		out = append(out, s)
	}
	return out, nil
}
