package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/seeu/backend/internal/domain"
)

type FollowRepository struct {
	db *DB
}

func NewFollowRepository(db *DB) *FollowRepository {
	return &FollowRepository{db: db}
}

func (r *FollowRepository) Create(ctx context.Context, followerID, followingID string) error {
	_, err := r.db.Pool.Exec(ctx,
		`INSERT INTO follows (follower_id, following_id) VALUES ($1, $2)`,
		followerID, followingID)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrAlreadyFollowing
		}
		return fmt.Errorf("create follow: %w", err)
	}
	return nil
}

// CreateAtomic (BACK-1) — INSERT follow + bump counters в одной транзакции.
// Либо все три операции коммитятся, либо rollback оставляет счётчики
// консистентными. Заменяет старый flow в follow_service.Follow.
func (r *FollowRepository) CreateAtomic(ctx context.Context, followerID, followingID string) error {
	return r.db.WithTx(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`INSERT INTO follows (follower_id, following_id) VALUES ($1, $2)`,
			followerID, followingID)
		if err != nil {
			if isUniqueViolation(err) {
				return domain.ErrAlreadyFollowing
			}
			return fmt.Errorf("create follow: %w", err)
		}
		if _, err := tx.Exec(ctx,
			`UPDATE users SET followers_count = followers_count + 1 WHERE id = $1`,
			followingID); err != nil {
			return fmt.Errorf("bump followers: %w", err)
		}
		if _, err := tx.Exec(ctx,
			`UPDATE users SET following_count = following_count + 1 WHERE id = $1`,
			followerID); err != nil {
			return fmt.Errorf("bump following: %w", err)
		}
		return nil
	})
}

// DeleteAtomic (BACK-1) — обратная операция. Decrement обернут в GREATEST
// чтобы счётчики не уходили в negative при rare race-condition'ах.
func (r *FollowRepository) DeleteAtomic(ctx context.Context, followerID, followingID string) error {
	return r.db.WithTx(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx,
			`DELETE FROM follows WHERE follower_id = $1 AND following_id = $2`,
			followerID, followingID)
		if err != nil {
			return fmt.Errorf("delete follow: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrNotFollowing
		}
		if _, err := tx.Exec(ctx,
			`UPDATE users SET followers_count = GREATEST(followers_count - 1, 0) WHERE id = $1`,
			followingID); err != nil {
			return fmt.Errorf("dec followers: %w", err)
		}
		if _, err := tx.Exec(ctx,
			`UPDATE users SET following_count = GREATEST(following_count - 1, 0) WHERE id = $1`,
			followerID); err != nil {
			return fmt.Errorf("dec following: %w", err)
		}
		return nil
	})
}

func (r *FollowRepository) Delete(ctx context.Context, followerID, followingID string) error {
	result, err := r.db.Pool.Exec(ctx,
		`DELETE FROM follows WHERE follower_id = $1 AND following_id = $2`,
		followerID, followingID)
	if err != nil {
		return fmt.Errorf("delete follow: %w", err)
	}
	if result.RowsAffected() == 0 {
		return domain.ErrNotFollowing
	}
	return nil
}

// GetMutuals returns users that userID follows AND who follow userID back.
func (r *FollowRepository) GetMutuals(ctx context.Context, userID string) ([]*domain.User, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT u.id, u.username, u.phone, u.full_name, u.bio, u.avatar_url, u.website,
		       u.gender, u.date_of_birth, u.device_public_id, u.device_private_id,
		       u.is_private, u.is_verified, u.posts_count, u.followers_count, u.following_count,
		       u.last_seen_at, COALESCE(u.hide_last_seen, false),
		       COALESCE(u.channel_about, ''), COALESCE(u.channel_banner_url, ''),
		       u.created_at, u.updated_at
		FROM users u
		INNER JOIN follows f1 ON f1.following_id = u.id AND f1.follower_id = $1
		INNER JOIN follows f2 ON f2.follower_id = u.id AND f2.following_id = $1
		ORDER BY u.full_name`, userID)
	if err != nil {
		return nil, fmt.Errorf("get mutuals: %w", err)
	}
	defer rows.Close()
	var users []*domain.User
	for rows.Next() {
		u := &domain.User{}
		if err := rows.Scan(
			&u.ID, &u.Username, &u.Phone, &u.FullName, &u.Bio, &u.AvatarURL, &u.Website,
			&u.Gender, &u.DateOfBirth, &u.DevicePublicID, &u.DevicePrivateID,
			&u.IsPrivate, &u.IsVerified, &u.PostsCount, &u.FollowersCount, &u.FollowingCount,
			&u.LastSeenAt, &u.HideLastSeen, &u.ChannelAbout, &u.ChannelBannerURL,
			&u.CreatedAt, &u.UpdatedAt,
		); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (r *FollowRepository) IsFollowing(ctx context.Context, followerID, followingID string) (bool, error) {
	var exists bool
	err := r.db.Pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM follows WHERE follower_id = $1 AND following_id = $2)`,
		followerID, followingID).Scan(&exists)
	return exists, err
}

func (r *FollowRepository) GetFollowerIDs(ctx context.Context, userID string) ([]string, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT follower_id FROM follows WHERE following_id = $1`,
		userID)
	if err != nil {
		return nil, fmt.Errorf("get follower ids: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}

	return ids, rows.Err()
}

// GetFollowingIDs — кого фолловит userID (BUG-17 prereq для приватного
// BLE-резолва: searching по private_id ограничен whitelist'ом following'а).
func (r *FollowRepository) GetFollowingIDs(ctx context.Context, userID string) ([]string, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT following_id FROM follows WHERE follower_id = $1`,
		userID)
	if err != nil {
		return nil, fmt.Errorf("get following ids: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}

	return ids, rows.Err()
}
