package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/seeu/backend/internal/domain"
)

type HighlightRepository struct {
	db *DB
}

func NewHighlightRepository(db *DB) *HighlightRepository {
	return &HighlightRepository{db: db}
}

func (r *HighlightRepository) Create(ctx context.Context, h *domain.Highlight) error {
	query := `
		INSERT INTO highlights (user_id, title, cover_url)
		VALUES ($1, $2, $3)
		RETURNING id, created_at, updated_at`

	err := r.db.Pool.QueryRow(ctx, query,
		h.UserID,
		h.Title,
		h.CoverURL,
	).Scan(&h.ID, &h.CreatedAt, &h.UpdatedAt)

	if err != nil {
		return fmt.Errorf("create highlight: %w", err)
	}

	return nil
}

func (r *HighlightRepository) GetByID(ctx context.Context, id string) (*domain.Highlight, error) {
	query := `
		SELECT id, user_id, title, cover_url, created_at, updated_at
		FROM highlights WHERE id = $1`

	h := &domain.Highlight{}
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&h.ID, &h.UserID, &h.Title, &h.CoverURL, &h.CreatedAt, &h.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrHighlightNotFound
		}
		return nil, fmt.Errorf("get highlight by id: %w", err)
	}

	return h, nil
}

func (r *HighlightRepository) GetByUsername(ctx context.Context, username string) ([]*domain.Highlight, error) {
	query := `
		SELECT h.id, h.user_id, h.title, h.cover_url, h.created_at, h.updated_at
		FROM highlights h
		JOIN users u ON u.id = h.user_id
		WHERE u.username = $1
		ORDER BY h.created_at DESC`

	rows, err := r.db.Pool.Query(ctx, query, username)
	if err != nil {
		return nil, fmt.Errorf("get highlights by username: %w", err)
	}
	defer rows.Close()

	var highlights []*domain.Highlight
	for rows.Next() {
		h := &domain.Highlight{}
		if err := rows.Scan(
			&h.ID, &h.UserID, &h.Title, &h.CoverURL, &h.CreatedAt, &h.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan highlight: %w", err)
		}
		highlights = append(highlights, h)
	}

	return highlights, rows.Err()
}

func (r *HighlightRepository) Update(ctx context.Context, h *domain.Highlight) error {
	query := `
		UPDATE highlights
		SET title = $1, cover_url = $2, updated_at = NOW()
		WHERE id = $3 AND user_id = $4
		RETURNING updated_at`

	err := r.db.Pool.QueryRow(ctx, query,
		h.Title, h.CoverURL, h.ID, h.UserID,
	).Scan(&h.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrHighlightNotFound
		}
		return fmt.Errorf("update highlight: %w", err)
	}

	return nil
}

func (r *HighlightRepository) Delete(ctx context.Context, id, userID string) error {
	result, err := r.db.Pool.Exec(ctx,
		`DELETE FROM highlights WHERE id = $1 AND user_id = $2`,
		id, userID)
	if err != nil {
		return fmt.Errorf("delete highlight: %w", err)
	}
	if result.RowsAffected() == 0 {
		return domain.ErrHighlightNotFound
	}
	return nil
}

func (r *HighlightRepository) AddStory(ctx context.Context, highlightID, storyID string) error {
	_, err := r.db.Pool.Exec(ctx,
		`INSERT INTO highlight_stories (highlight_id, story_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		highlightID, storyID)
	return err
}

func (r *HighlightRepository) RemoveStory(ctx context.Context, highlightID, storyID string) error {
	_, err := r.db.Pool.Exec(ctx,
		`DELETE FROM highlight_stories WHERE highlight_id = $1 AND story_id = $2`,
		highlightID, storyID)
	return err
}

func (r *HighlightRepository) ReplaceStories(ctx context.Context, highlightID string, storyIDs []string) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `DELETE FROM highlight_stories WHERE highlight_id = $1`, highlightID)
	if err != nil {
		return err
	}

	for _, storyID := range storyIDs {
		_, err = tx.Exec(ctx,
			`INSERT INTO highlight_stories (highlight_id, story_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			highlightID, storyID)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (r *HighlightRepository) GetStories(ctx context.Context, highlightID string) ([]*domain.Story, error) {
	query := `
		SELECT s.id, s.user_id, COALESCE(s.media_url, ''), s.media_type, s.duration, s.text_overlay,
		       s.views_count, s.likes_count, s.audio_track_id, COALESCE(s.bg_color, ''), s.poll, COALESCE(s.is_close_friends_only, false), s.expires_at, s.created_at,
		       u.id, u.username, u.full_name, u.avatar_url, u.is_verified
		FROM stories s
		JOIN highlight_stories hs ON hs.story_id = s.id
		JOIN users u ON u.id = s.user_id
		WHERE hs.highlight_id = $1
		ORDER BY hs.added_at ASC`

	rows, err := r.db.Pool.Query(ctx, query, highlightID)
	if err != nil {
		return nil, fmt.Errorf("get highlight stories: %w", err)
	}
	defer rows.Close()

	var stories []*domain.Story
	for rows.Next() {
		s := &domain.Story{User: &domain.UserShort{}}
		var pollRaw []byte
		if err := rows.Scan(
			&s.ID, &s.UserID, &s.MediaURL, &s.MediaType,
			&s.Duration, &s.TextOverlay, &s.ViewsCount, &s.LikesCount,
			&s.AudioTrackID, &s.BgColor, &pollRaw, &s.IsCloseFriendsOnly, &s.ExpiresAt, &s.CreatedAt,
			&s.User.ID, &s.User.Username, &s.User.FullName,
			&s.User.AvatarURL, &s.User.IsVerified,
		); err != nil {
			return nil, fmt.Errorf("scan story: %w", err)
		}
		if len(pollRaw) > 0 {
			var p domain.StoryPoll
			if err := json.Unmarshal(pollRaw, &p); err == nil {
				s.Poll = &p
			}
		}
		stories = append(stories, s)
	}

	return stories, rows.Err()
}
