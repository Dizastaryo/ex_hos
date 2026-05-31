package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/seeu/backend/internal/domain"
)

// FollowRequest represents a pending follow request to a private account.
type FollowRequest struct {
	ID          string    `json:"id"`
	RequesterID string    `json:"requester_id"`
	TargetID    string    `json:"target_id"`
	CreatedAt   time.Time `json:"created_at"`

	// Hydrated fields for the inbox view.
	Requester *RequesterShort `json:"requester,omitempty"`
}

type RequesterShort struct {
	ID         string `json:"id"`
	Username   string `json:"username"`
	FullName   string `json:"full_name"`
	AvatarURL  string `json:"avatar_url"`
	IsVerified bool   `json:"is_verified"`
}

type FollowRequestRepository struct {
	db *DB
}

func NewFollowRequestRepository(db *DB) *FollowRequestRepository {
	return &FollowRequestRepository{db: db}
}

// Create returns ErrAlreadyExists when the same pair already has a pending row.
func (r *FollowRequestRepository) Create(ctx context.Context, requesterID, targetID string) (*FollowRequest, error) {
	row := &FollowRequest{}
	err := r.db.Pool.QueryRow(ctx, `
		INSERT INTO follow_requests (requester_id, target_id)
		VALUES ($1, $2)
		RETURNING id, requester_id, target_id, created_at`,
		requesterID, targetID,
	).Scan(&row.ID, &row.RequesterID, &row.TargetID, &row.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, domain.ErrAlreadyExists
		}
		return nil, fmt.Errorf("create follow request: %w", err)
	}
	return row, nil
}

// HasPending checks whether requester has an open request to target.
func (r *FollowRequestRepository) HasPending(ctx context.Context, requesterID, targetID string) (bool, error) {
	var exists bool
	err := r.db.Pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM follow_requests
		              WHERE requester_id = $1 AND target_id = $2)`,
		requesterID, targetID).Scan(&exists)
	return exists, err
}

// Get loads a single request by id (for accept/decline auth).
func (r *FollowRequestRepository) GetByID(ctx context.Context, id string) (*FollowRequest, error) {
	row := &FollowRequest{}
	err := r.db.Pool.QueryRow(ctx, `
		SELECT id, requester_id, target_id, created_at
		FROM follow_requests WHERE id = $1`, id).Scan(
		&row.ID, &row.RequesterID, &row.TargetID, &row.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return row, nil
}

// Delete by id, returns ErrNotFound if missing.
func (r *FollowRequestRepository) Delete(ctx context.Context, id string) error {
	tag, err := r.db.Pool.Exec(ctx, `DELETE FROM follow_requests WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete follow request: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// DeletePair — convenience for unfollow flow when target is still private.
func (r *FollowRequestRepository) DeletePair(ctx context.Context, requesterID, targetID string) error {
	_, err := r.db.Pool.Exec(ctx, `
		DELETE FROM follow_requests WHERE requester_id = $1 AND target_id = $2`,
		requesterID, targetID)
	return err
}

// ListForTarget returns pending requests to target, with requester info.
func (r *FollowRequestRepository) ListForTarget(ctx context.Context, targetID string, limit, offset int) ([]*FollowRequest, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.db.Pool.Query(ctx, `
		SELECT fr.id, fr.requester_id, fr.target_id, fr.created_at,
		       u.id, u.username, u.full_name, u.avatar_url, u.is_verified
		FROM follow_requests fr
		JOIN users u ON u.id = fr.requester_id
		WHERE fr.target_id = $1
		ORDER BY fr.created_at DESC
		LIMIT $2 OFFSET $3`, targetID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list follow requests: %w", err)
	}
	defer rows.Close()

	var out []*FollowRequest
	for rows.Next() {
		fr := &FollowRequest{Requester: &RequesterShort{}}
		if err := rows.Scan(
			&fr.ID, &fr.RequesterID, &fr.TargetID, &fr.CreatedAt,
			&fr.Requester.ID, &fr.Requester.Username, &fr.Requester.FullName,
			&fr.Requester.AvatarURL, &fr.Requester.IsVerified,
		); err != nil {
			return nil, err
		}
		out = append(out, fr)
	}
	return out, nil
}
