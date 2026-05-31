package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/seeu/backend/internal/domain"
)

// RestrictionRepository — PROFILE-4. Restrict-feature (Insta-like): user_id
// «ограничивает» restricted_user_id. Этот юзер может оставлять комменты на
// постах автора, но они видны только ему самому и автору поста — другие
// зрители их не видят. Менее агрессивный чем full-block.
type RestrictionRepository struct {
	db *DB
}

func NewRestrictionRepository(db *DB) *RestrictionRepository {
	return &RestrictionRepository{db: db}
}

// Create — idempotent. CHECK на self-restrict в SQL.
func (r *RestrictionRepository) Create(ctx context.Context, userID, restrictedID string) error {
	if userID == restrictedID {
		return domain.ErrInvalidInput
	}
	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO user_restrictions (user_id, restricted_user_id)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING`, userID, restrictedID)
	if err != nil {
		return fmt.Errorf("create restriction: %w", err)
	}
	return nil
}

func (r *RestrictionRepository) Delete(ctx context.Context, userID, restrictedID string) error {
	_, err := r.db.Pool.Exec(ctx,
		`DELETE FROM user_restrictions WHERE user_id = $1 AND restricted_user_id = $2`,
		userID, restrictedID)
	if err != nil {
		return fmt.Errorf("delete restriction: %w", err)
	}
	return nil
}

// IsRestricted — true если userID ограничил restrictedID. НЕ симметрично
// (в отличие от blocks).
func (r *RestrictionRepository) IsRestricted(ctx context.Context, userID, restrictedID string) (bool, error) {
	var n int
	err := r.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM user_restrictions
		WHERE user_id = $1 AND restricted_user_id = $2`, userID, restrictedID).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("check restriction: %w", err)
	}
	return n > 0, nil
}

type RestrictedUserSummary struct {
	UserID       string    `json:"user_id"`
	Username     string    `json:"username"`
	FullName     string    `json:"full_name"`
	AvatarURL    string    `json:"avatar_url"`
	RestrictedAt time.Time `json:"restricted_at"`
}

func (r *RestrictionRepository) List(ctx context.Context, userID string, limit, offset int) ([]*RestrictedUserSummary, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.Pool.Query(ctx, `
		SELECT u.id, u.username, u.full_name, u.avatar_url, r.created_at
		FROM user_restrictions r
		JOIN users u ON u.id = r.restricted_user_id
		WHERE r.user_id = $1
		ORDER BY r.created_at DESC
		LIMIT $2 OFFSET $3`, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list restricted: %w", err)
	}
	defer rows.Close()

	var out []*RestrictedUserSummary
	for rows.Next() {
		s := &RestrictedUserSummary{}
		if err := rows.Scan(&s.UserID, &s.Username, &s.FullName, &s.AvatarURL, &s.RestrictedAt); err != nil {
			return nil, fmt.Errorf("scan restricted: %w", err)
		}
		out = append(out, s)
	}
	return out, nil
}
