package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type SearchHistoryRepository struct {
	db *DB
}

func NewSearchHistoryRepository(db *DB) *SearchHistoryRepository {
	return &SearchHistoryRepository{db: db}
}

// SearchHistoryItem is one row exposed to clients.
type SearchHistoryItem struct {
	Query     string    `json:"query"`
	CreatedAt time.Time `json:"created_at"`
}

// Record upserts the (user, query) pair, bumping created_at on conflict.
// Cleans the query (trim, lowercase) so "Foo" and "foo " collapse to one row.
// Empty/whitespace queries are silently ignored.
func (r *SearchHistoryRepository) Record(ctx context.Context, userID, query string) error {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return nil
	}
	if len(q) > 120 {
		q = q[:120]
	}
	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO search_history (user_id, query)
		VALUES ($1, $2)
		ON CONFLICT (user_id, query)
		DO UPDATE SET created_at = NOW()`,
		userID, q,
	)
	if err != nil {
		return fmt.Errorf("record search history: %w", err)
	}
	return nil
}

// List returns the user's recent searches, newest first. Default cap = 10.
func (r *SearchHistoryRepository) List(ctx context.Context, userID string, limit int) ([]SearchHistoryItem, error) {
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	rows, err := r.db.Pool.Query(ctx, `
		SELECT query, created_at
		FROM search_history
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("list search history: %w", err)
	}
	defer rows.Close()

	var out []SearchHistoryItem
	for rows.Next() {
		var item SearchHistoryItem
		if err := rows.Scan(&item.Query, &item.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

// DeleteOne removes a single query for the user. No-op if absent.
func (r *SearchHistoryRepository) DeleteOne(ctx context.Context, userID, query string) error {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return nil
	}
	_, err := r.db.Pool.Exec(ctx,
		`DELETE FROM search_history WHERE user_id = $1 AND query = $2`,
		userID, q)
	return err
}

// Clear nukes the user's whole history.
func (r *SearchHistoryRepository) Clear(ctx context.Context, userID string) error {
	_, err := r.db.Pool.Exec(ctx,
		`DELETE FROM search_history WHERE user_id = $1`, userID)
	return err
}
