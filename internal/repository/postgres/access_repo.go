package postgres

import (
	"context"
	"fmt"

	"github.com/seeu/backend/internal/domain"
)

type AccessRepository struct {
	db *DB
}

func NewAccessRepository(db *DB) *AccessRepository {
	return &AccessRepository{db: db}
}

// normalize returns (a, b) such that a < b (UUID string comparison).
func normalize(userAID, userBID string) (string, string) {
	if userAID < userBID {
		return userAID, userBID
	}
	return userBID, userAID
}

// GrantAccess creates a mutual access record between two users.
// Idempotent: does nothing if record already exists.
func (r *AccessRepository) GrantAccess(ctx context.Context, userAID, userBID string) error {
	a, b := normalize(userAID, userBID)
	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO user_access (user_a_id, user_b_id)
		VALUES ($1, $2)
		ON CONFLICT (user_a_id, user_b_id) DO NOTHING`,
		a, b,
	)
	if err != nil {
		return fmt.Errorf("grant access: %w", err)
	}
	return nil
}

// HasAccess checks whether two users have mutual access.
func (r *AccessRepository) HasAccess(ctx context.Context, userAID, userBID string) (bool, error) {
	a, b := normalize(userAID, userBID)
	var exists bool
	err := r.db.Pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM user_access
			WHERE user_a_id = $1 AND user_b_id = $2
		)`, a, b,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("has access: %w", err)
	}
	return exists, nil
}

// RevokeAccess removes the access record between two users.
func (r *AccessRepository) RevokeAccess(ctx context.Context, userAID, userBID string) error {
	a, b := normalize(userAID, userBID)
	_, err := r.db.Pool.Exec(ctx, `
		DELETE FROM user_access WHERE user_a_id = $1 AND user_b_id = $2`,
		a, b,
	)
	if err != nil {
		return fmt.Errorf("revoke access: %w", err)
	}
	return nil
}

// ListAccessPartners returns all users who have mutual access with userID.
func (r *AccessRepository) ListAccessPartners(ctx context.Context, userID string, limit, offset int) ([]*domain.AccessPartner, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT u.id, u.username, u.full_name, COALESCE(u.avatar_url, ''), COALESCE(u.is_verified, false), ua.granted_at
		FROM user_access ua
		JOIN users u ON (
			CASE WHEN ua.user_a_id = $1 THEN ua.user_b_id ELSE ua.user_a_id END = u.id
		)
		WHERE ua.user_a_id = $1 OR ua.user_b_id = $1
		ORDER BY ua.granted_at DESC
		LIMIT $2 OFFSET $3`,
		userID, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("list access partners: %w", err)
	}
	defer rows.Close()

	var result []*domain.AccessPartner
	for rows.Next() {
		p := &domain.AccessPartner{}
		if err := rows.Scan(&p.UserID, &p.Username, &p.FullName, &p.AvatarURL, &p.IsVerified, &p.GrantedAt); err != nil {
			return nil, fmt.Errorf("scan access partner: %w", err)
		}
		result = append(result, p)
	}
	return result, nil
}
