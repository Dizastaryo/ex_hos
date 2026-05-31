package postgres

import (
	"context"
	"fmt"
	"time"
)

// AIMask — record в таблице ai_masks. Создаётся когда юзер успешно
// генерит маску через `POST /ai/mask`. file_url — путь к скачанному PNG
// в /uploads/ai/masks/, который мы держим у себя (DALL-E URLs expir'ятся).
type AIMask struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Prompt    string    `json:"prompt"`
	FileURL   string    `json:"file_url"`
	CreatedAt time.Time `json:"created_at"`
}

type AIMasksRepository struct {
	db *DB
}

func NewAIMasksRepository(db *DB) *AIMasksRepository {
	return &AIMasksRepository{db: db}
}

// Insert создаёт row и возвращает id + created_at — для немедленного
// возврата клиенту без дополнительного round-trip'а.
func (r *AIMasksRepository) Insert(ctx context.Context, userID, prompt, fileURL string) (AIMask, error) {
	var m AIMask
	err := r.db.Pool.QueryRow(ctx, `
		INSERT INTO ai_masks (user_id, prompt, file_url)
		VALUES ($1, $2, $3)
		RETURNING id, user_id, prompt, file_url, created_at`,
		userID, prompt, fileURL,
	).Scan(&m.ID, &m.UserID, &m.Prompt, &m.FileURL, &m.CreatedAt)
	if err != nil {
		return AIMask{}, fmt.Errorf("insert ai_mask: %w", err)
	}
	return m, nil
}

// ListByUser возвращает историю масок юзера, свежие сверху.
func (r *AIMasksRepository) ListByUser(ctx context.Context, userID string, limit int) ([]AIMask, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, user_id, prompt, file_url, created_at
		FROM ai_masks
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2`,
		userID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list ai_masks: %w", err)
	}
	defer rows.Close()
	var out []AIMask
	for rows.Next() {
		var m AIMask
		if err := rows.Scan(&m.ID, &m.UserID, &m.Prompt, &m.FileURL, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan ai_mask: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// CountInLast24h — для rate-limit'а (5 generations/day на юзера).
func (r *AIMasksRepository) CountInLast24h(ctx context.Context, userID string) (int, error) {
	var n int
	err := r.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM ai_masks
		WHERE user_id = $1 AND created_at > NOW() - INTERVAL '24 hours'`,
		userID,
	).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count ai_masks: %w", err)
	}
	return n, nil
}

// Delete удаляет mask и возвращает file_url чтобы caller почистил blob с диска.
func (r *AIMasksRepository) Delete(ctx context.Context, id, userID string) (string, error) {
	var fileURL string
	err := r.db.Pool.QueryRow(ctx, `
		DELETE FROM ai_masks WHERE id = $1 AND user_id = $2
		RETURNING file_url`,
		id, userID,
	).Scan(&fileURL)
	if err != nil {
		return "", fmt.Errorf("delete ai_mask: %w", err)
	}
	return fileURL, nil
}
