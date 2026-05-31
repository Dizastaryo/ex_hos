package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/seeu/backend/internal/domain"
)

// CloseFriendsRepository — PROFILE-3. owner_id выбирает подмножество юзеров
// как «close friends». Stories с is_close_friends_only=true видят только они.
// Симметричен block_repo по структуре.
type CloseFriendsRepository struct {
	db *DB
}

func NewCloseFriendsRepository(db *DB) *CloseFriendsRepository {
	return &CloseFriendsRepository{db: db}
}

// Add — idempotent. Owner добавляет friend в CF список.
func (r *CloseFriendsRepository) Add(ctx context.Context, ownerID, friendID string) error {
	if ownerID == friendID {
		return domain.ErrInvalidInput
	}
	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO close_friends (owner_id, friend_id)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING`, ownerID, friendID)
	if err != nil {
		return fmt.Errorf("add close friend: %w", err)
	}
	return nil
}

func (r *CloseFriendsRepository) Remove(ctx context.Context, ownerID, friendID string) error {
	_, err := r.db.Pool.Exec(ctx,
		`DELETE FROM close_friends WHERE owner_id = $1 AND friend_id = $2`,
		ownerID, friendID)
	if err != nil {
		return fmt.Errorf("remove close friend: %w", err)
	}
	return nil
}

// IsCloseFriend — true если ownerID добавил friendID в свой CF список.
// Не симметрично: A в CF у B не означает B в CF у A.
func (r *CloseFriendsRepository) IsCloseFriend(ctx context.Context, ownerID, friendID string) (bool, error) {
	var n int
	err := r.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM close_friends
		WHERE owner_id = $1 AND friend_id = $2`, ownerID, friendID).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("check close friend: %w", err)
	}
	return n > 0, nil
}

type CloseFriendSummary struct {
	UserID    string    `json:"user_id"`
	Username  string    `json:"username"`
	FullName  string    `json:"full_name"`
	AvatarURL string    `json:"avatar_url"`
	AddedAt   time.Time `json:"added_at"`
}

func (r *CloseFriendsRepository) List(ctx context.Context, ownerID string, limit, offset int) ([]*CloseFriendSummary, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.db.Pool.Query(ctx, `
		SELECT u.id, u.username, u.full_name, u.avatar_url, cf.created_at
		FROM close_friends cf
		JOIN users u ON u.id = cf.friend_id
		WHERE cf.owner_id = $1
		ORDER BY cf.created_at DESC
		LIMIT $2 OFFSET $3`, ownerID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list close friends: %w", err)
	}
	defer rows.Close()

	var out []*CloseFriendSummary
	for rows.Next() {
		s := &CloseFriendSummary{}
		if err := rows.Scan(&s.UserID, &s.Username, &s.FullName, &s.AvatarURL, &s.AddedAt); err != nil {
			return nil, fmt.Errorf("scan close friend: %w", err)
		}
		out = append(out, s)
	}
	return out, nil
}
