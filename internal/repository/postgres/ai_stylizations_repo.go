package postgres

import (
	"context"
	"fmt"
	"time"
)

// AIStylization — запись о стилизованном фото.
type AIStylization struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	SourceURL string    `json:"source_url"`
	ResultURL string    `json:"result_url"`
	Style     string    `json:"style"`
	Prompt    string    `json:"prompt"`
	CreatedAt time.Time `json:"created_at"`
}

type AIStylizationsRepository struct {
	db *DB
}

func NewAIStylizationsRepository(db *DB) *AIStylizationsRepository {
	return &AIStylizationsRepository{db: db}
}

func (r *AIStylizationsRepository) Insert(ctx context.Context, userID, sourceURL, resultURL, style, prompt string) (AIStylization, error) {
	var s AIStylization
	err := r.db.Pool.QueryRow(ctx, `
		INSERT INTO ai_stylizations (user_id, source_url, result_url, style, prompt)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, user_id, source_url, result_url, style, prompt, created_at`,
		userID, sourceURL, resultURL, style, prompt,
	).Scan(&s.ID, &s.UserID, &s.SourceURL, &s.ResultURL, &s.Style, &s.Prompt, &s.CreatedAt)
	if err != nil {
		return AIStylization{}, fmt.Errorf("insert ai_stylization: %w", err)
	}
	return s, nil
}

// CountInLast24h — для rate-limit'а (3 stylize/day).
func (r *AIStylizationsRepository) CountInLast24h(ctx context.Context, userID string) (int, error) {
	var n int
	err := r.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM ai_stylizations
		WHERE user_id = $1 AND created_at > NOW() - INTERVAL '24 hours'`,
		userID,
	).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count ai_stylizations: %w", err)
	}
	return n, nil
}

func (r *AIStylizationsRepository) ListByUser(ctx context.Context, userID string, limit int) ([]AIStylization, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, user_id, source_url, result_url, style, prompt, created_at
		FROM ai_stylizations
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2`,
		userID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list ai_stylizations: %w", err)
	}
	defer rows.Close()
	var out []AIStylization
	for rows.Next() {
		var s AIStylization
		if err := rows.Scan(&s.ID, &s.UserID, &s.SourceURL, &s.ResultURL, &s.Style, &s.Prompt, &s.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan ai_stylization: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
