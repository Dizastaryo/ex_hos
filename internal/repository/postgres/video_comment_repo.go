package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/seeu/backend/internal/domain"
)

type VideoComment struct {
	ID        string             `json:"id"`
	VideoID   string             `json:"video_id"`
	UserID    string             `json:"user_id"`
	Text      string             `json:"text"`
	CreatedAt time.Time          `json:"created_at"`
	User      *domain.UserShort  `json:"user,omitempty"`
}

type VideoCommentRepository struct {
	db *DB
}

func NewVideoCommentRepository(db *DB) *VideoCommentRepository {
	return &VideoCommentRepository{db: db}
}

// Create inserts a new comment and returns the populated row.
func (r *VideoCommentRepository) Create(ctx context.Context, videoID, userID, text string) (*VideoComment, error) {
	c := &VideoComment{User: &domain.UserShort{}}
	err := r.db.Pool.QueryRow(ctx, `
		WITH inserted AS (
			INSERT INTO video_comments (video_id, user_id, text)
			VALUES ($1, $2, $3)
			RETURNING id, video_id, user_id, text, created_at
		)
		SELECT i.id, i.video_id, i.user_id, i.text, i.created_at,
		       u.id, u.username, u.full_name, u.avatar_url, u.is_verified
		FROM inserted i
		JOIN users u ON u.id = i.user_id`,
		videoID, userID, text,
	).Scan(&c.ID, &c.VideoID, &c.UserID, &c.Text, &c.CreatedAt,
		&c.User.ID, &c.User.Username, &c.User.FullName, &c.User.AvatarURL, &c.User.IsVerified)
	if err != nil {
		return nil, fmt.Errorf("create video comment: %w", err)
	}

	// Bump denormalized counter on the video itself.
	if _, err := r.db.Pool.Exec(ctx,
		`UPDATE videos SET comments_count = comments_count + 1 WHERE id = $1`,
		videoID); err != nil {
		// Non-fatal; counter drift can be reconciled later.
		_ = err
	}

	return c, nil
}

// List returns the most recent comments for a video, newest first.
func (r *VideoCommentRepository) List(ctx context.Context, videoID string, limit, offset int) ([]*VideoComment, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.Pool.Query(ctx, `
		SELECT c.id, c.video_id, c.user_id, c.text, c.created_at,
		       u.id, u.username, u.full_name, u.avatar_url, u.is_verified
		FROM video_comments c
		JOIN users u ON u.id = c.user_id
		WHERE c.video_id = $1
		ORDER BY c.created_at DESC
		LIMIT $2 OFFSET $3`, videoID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list video comments: %w", err)
	}
	defer rows.Close()

	var out []*VideoComment
	for rows.Next() {
		c := &VideoComment{User: &domain.UserShort{}}
		if err := rows.Scan(&c.ID, &c.VideoID, &c.UserID, &c.Text, &c.CreatedAt,
			&c.User.ID, &c.User.Username, &c.User.FullName, &c.User.AvatarURL, &c.User.IsVerified); err != nil {
			return nil, fmt.Errorf("scan video comment: %w", err)
		}
		out = append(out, c)
	}
	return out, nil
}

// Delete removes a comment if the caller is its author. Decrements the video
// counter on success.
func (r *VideoCommentRepository) Delete(ctx context.Context, commentID, userID string) error {
	var videoID string
	err := r.db.Pool.QueryRow(ctx,
		`DELETE FROM video_comments WHERE id = $1 AND user_id = $2 RETURNING video_id`,
		commentID, userID).Scan(&videoID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		return fmt.Errorf("delete video comment: %w", err)
	}
	if _, err := r.db.Pool.Exec(ctx,
		`UPDATE videos SET comments_count = GREATEST(comments_count - 1, 0) WHERE id = $1`,
		videoID); err != nil {
		_ = err
	}
	return nil
}
