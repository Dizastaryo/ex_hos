package postgres

import (
	"context"
	"fmt"

	"github.com/seeu/backend/internal/domain"
)

type UserStatsRepository struct {
	db *DB
}

func NewUserStatsRepository(db *DB) *UserStatsRepository {
	return &UserStatsRepository{db: db}
}

// GetByUserID возвращает stats пользователя.
// Если строки нет — создаёт пустую (на случай если миграция не инициализировала).
func (r *UserStatsRepository) GetByUserID(ctx context.Context, userID string) (*domain.UserStats, error) {
	s := &domain.UserStats{}
	err := r.db.Pool.QueryRow(ctx, `
		SELECT user_id, total_likes, scanner_likes, post_likes, story_likes,
		       reel_likes, audio_likes, video_likes, book_likes, updated_at
		FROM user_stats
		WHERE user_id = $1`,
		userID,
	).Scan(
		&s.UserID, &s.TotalLikes, &s.ScannerLikes, &s.PostLikes, &s.StoryLikes,
		&s.ReelLikes, &s.AudioLikes, &s.VideoLikes, &s.BookLikes, &s.UpdatedAt,
	)
	if err != nil {
		// Строки нет — создаём и возвращаем нули
		if _, insertErr := r.db.Pool.Exec(ctx,
			`INSERT INTO user_stats (user_id) VALUES ($1) ON CONFLICT DO NOTHING`,
			userID,
		); insertErr != nil {
			return nil, fmt.Errorf("init user_stats: %w", insertErr)
		}
		s.UserID = userID
		return s, nil
	}
	return s, nil
}

// IncrementLikes атомарно увеличивает нужный счётчик + total_likes на 1.
// field — одно из: scanner_likes, post_likes, story_likes, reel_likes,
//
//	audio_likes, video_likes, book_likes.
//
// Строка создаётся если её ещё нет (upsert).
func (r *UserStatsRepository) IncrementLikes(ctx context.Context, userID, field string) error {
	// Допустимые поля — чтобы не было SQL-инъекции через format string.
	allowed := map[string]bool{
		"scanner_likes": true,
		"post_likes":    true,
		"story_likes":   true,
		"reel_likes":    true,
		"audio_likes":   true,
		"video_likes":   true,
		"book_likes":    true,
	}
	if !allowed[field] {
		return fmt.Errorf("user_stats: unknown field %q", field)
	}

	q := fmt.Sprintf(`
		INSERT INTO user_stats (user_id, %s, total_likes)
		VALUES ($1, 1, 1)
		ON CONFLICT (user_id) DO UPDATE
		SET %s       = user_stats.%s + 1,
		    total_likes = user_stats.total_likes + 1,
		    updated_at  = NOW()`,
		field, field, field)

	_, err := r.db.Pool.Exec(ctx, q, userID)
	if err != nil {
		return fmt.Errorf("increment %s for user %s: %w", field, userID, err)
	}
	return nil
}

// TopUsers возвращает топ-N пользователей по total_likes для leaderboard.
func (r *UserStatsRepository) TopUsers(ctx context.Context, limit int) ([]*domain.LeaderboardEntry, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := r.db.Pool.Query(ctx, `
		SELECT ROW_NUMBER() OVER (ORDER BY s.total_likes DESC) AS rank,
		       u.id, u.username, COALESCE(u.full_name,''), COALESCE(u.avatar_url,''),
		       s.total_likes
		FROM user_stats s
		JOIN users u ON u.id = s.user_id
		WHERE s.total_likes > 0
		ORDER BY s.total_likes DESC
		LIMIT $1`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("top users: %w", err)
	}
	defer rows.Close()
	var out []*domain.LeaderboardEntry
	for rows.Next() {
		e := &domain.LeaderboardEntry{}
		if err := rows.Scan(&e.Rank, &e.UserID, &e.Username, &e.FullName, &e.AvatarURL, &e.TotalLikes); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// DecrementLikes атомарно уменьшает счётчик + total_likes, не уходя в минус.
func (r *UserStatsRepository) DecrementLikes(ctx context.Context, userID, field string) error {
	allowed := map[string]bool{
		"scanner_likes": true,
		"post_likes":    true,
		"story_likes":   true,
		"reel_likes":    true,
		"audio_likes":   true,
		"video_likes":   true,
		"book_likes":    true,
	}
	if !allowed[field] {
		return fmt.Errorf("user_stats: unknown field %q", field)
	}

	q := fmt.Sprintf(`
		UPDATE user_stats
		SET %s       = GREATEST(%s - 1, 0),
		    total_likes = GREATEST(total_likes - 1, 0),
		    updated_at  = NOW()
		WHERE user_id = $1`,
		field, field)

	_, err := r.db.Pool.Exec(ctx, q, userID)
	return err
}
