package postgres

import (
	"context"
	"fmt"
	"time"
)

type StickerRow struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	URL       string    `json:"url"`
	CreatedAt time.Time `json:"created_at"`
}

type StickerRepository struct {
	db *DB
}

func NewStickerRepository(db *DB) *StickerRepository {
	return &StickerRepository{db: db}
}

func (r *StickerRepository) Create(ctx context.Context, userID, url string) (StickerRow, error) {
	var s StickerRow
	err := r.db.Pool.QueryRow(ctx,
		`INSERT INTO stickers (user_id, url)
		 VALUES ($1, $2)
		 RETURNING id, user_id, url, created_at`,
		userID, url,
	).Scan(&s.ID, &s.UserID, &s.URL, &s.CreatedAt)
	return s, err
}

func (r *StickerRepository) ListByUser(ctx context.Context, userID string) ([]StickerRow, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, user_id, url, created_at
		 FROM stickers
		 WHERE user_id = $1
		 ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list stickers: %w", err)
	}
	defer rows.Close()
	var stickers []StickerRow
	for rows.Next() {
		var s StickerRow
		if err := rows.Scan(&s.ID, &s.UserID, &s.URL, &s.CreatedAt); err != nil {
			return nil, err
		}
		stickers = append(stickers, s)
	}
	return stickers, rows.Err()
}

func (r *StickerRepository) Delete(ctx context.Context, id, userID string) (bool, error) {
	tag, err := r.db.Pool.Exec(ctx,
		`DELETE FROM stickers WHERE id = $1 AND user_id = $2`,
		id, userID,
	)
	if err != nil {
		return false, fmt.Errorf("delete sticker: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}
