package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/seeu/backend/internal/domain"
)

type LikeRepository struct {
	db *DB
}

func NewLikeRepository(db *DB) *LikeRepository {
	return &LikeRepository{db: db}
}

func (r *LikeRepository) Create(ctx context.Context, like *domain.Like) error {
	query := `
		INSERT INTO likes (user_id, entity_id, entity_type)
		VALUES ($1, $2, $3)
		RETURNING id, created_at`

	err := r.db.Pool.QueryRow(ctx, query,
		like.UserID,
		like.EntityID,
		like.EntityType,
	).Scan(&like.ID, &like.CreatedAt)

	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrAlreadyLiked
		}
		return fmt.Errorf("create like: %w", err)
	}

	return nil
}

func (r *LikeRepository) Delete(ctx context.Context, userID, entityID, entityType string) error {
	result, err := r.db.Pool.Exec(ctx,
		`DELETE FROM likes WHERE user_id = $1 AND entity_id = $2 AND entity_type = $3`,
		userID, entityID, entityType)
	if err != nil {
		return fmt.Errorf("delete like: %w", err)
	}
	if result.RowsAffected() == 0 {
		return domain.ErrNotLiked
	}
	return nil
}

func (r *LikeRepository) Exists(ctx context.Context, userID, entityID, entityType string) (bool, error) {
	var exists bool
	err := r.db.Pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM likes WHERE user_id = $1 AND entity_id = $2 AND entity_type = $3)`,
		userID, entityID, entityType).Scan(&exists)
	return exists, err
}

// LikePostAtomic / UnlikePostAtomic (BACK-1) — INSERT/DELETE like + bump
// posts.likes_count в одной транзакции. counterTable выбирается caller'ом
// чтобы один helper покрыл posts/comments/videos. Counters клампятся через
// GREATEST на decrement.
func (r *LikeRepository) LikeEntityAtomic(
	ctx context.Context, userID, entityID, entityType, counterTable string,
) error {
	return r.db.WithTx(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`INSERT INTO likes (user_id, entity_id, entity_type) VALUES ($1, $2, $3)`,
			userID, entityID, entityType)
		if err != nil {
			if isUniqueViolation(err) {
				return domain.ErrAlreadyLiked
			}
			return fmt.Errorf("create like: %w", err)
		}
		// counterTable — посчитан caller'ом (posts/comments/videos), inject'ится
		// через formatted SQL так как pg не parametrize'ит table names.
		// Безопасно потому что параметр приходит от service-layer а не от user.
		q := fmt.Sprintf(
			`UPDATE %s SET likes_count = likes_count + 1 WHERE id = $1`, counterTable)
		if _, err := tx.Exec(ctx, q, entityID); err != nil {
			return fmt.Errorf("bump %s.likes_count: %w", counterTable, err)
		}
		return nil
	})
}

func (r *LikeRepository) UnlikeEntityAtomic(
	ctx context.Context, userID, entityID, entityType, counterTable string,
) error {
	return r.db.WithTx(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx,
			`DELETE FROM likes WHERE user_id = $1 AND entity_id = $2 AND entity_type = $3`,
			userID, entityID, entityType)
		if err != nil {
			return fmt.Errorf("delete like: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrNotLiked
		}
		q := fmt.Sprintf(
			`UPDATE %s SET likes_count = GREATEST(likes_count - 1, 0) WHERE id = $1`,
			counterTable)
		if _, err := tx.Exec(ctx, q, entityID); err != nil {
			return fmt.Errorf("dec %s.likes_count: %w", counterTable, err)
		}
		return nil
	})
}

func (r *LikeRepository) GetEntityOwnerID(ctx context.Context, entityID, entityType string) (string, error) {
	var ownerID string
	var err error

	switch entityType {
	case domain.LikeEntityPost:
		err = r.db.Pool.QueryRow(ctx,
			`SELECT user_id FROM posts WHERE id = $1`, entityID).Scan(&ownerID)
	case domain.LikeEntityComment:
		err = r.db.Pool.QueryRow(ctx,
			`SELECT user_id FROM comments WHERE id = $1`, entityID).Scan(&ownerID)
	case domain.LikeEntityStory:
		err = r.db.Pool.QueryRow(ctx,
			`SELECT user_id FROM stories WHERE id = $1`, entityID).Scan(&ownerID)
	default:
		return "", domain.ErrInvalidInput
	}

	return ownerID, err
}
