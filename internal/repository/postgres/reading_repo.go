package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/seeu/backend/internal/domain"
)

type ReadingRepository struct {
	db *DB
}

func NewReadingRepository(db *DB) *ReadingRepository {
	return &ReadingRepository{db: db}
}

func (r *ReadingRepository) GetProgress(ctx context.Context, userID, fileID string) (*domain.ReadingProgress, error) {
	p := &domain.ReadingProgress{}
	var pos []byte
	err := r.db.Pool.QueryRow(ctx,
		`SELECT user_id, file_id, position, last_read_at
		 FROM reading_progress WHERE user_id = $1 AND file_id = $2`,
		userID, fileID,
	).Scan(&p.UserID, &p.FileID, &pos, &p.LastReadAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get progress: %w", err)
	}
	p.Position = json.RawMessage(pos)
	return p, nil
}

func (r *ReadingRepository) UpsertProgress(ctx context.Context, userID, fileID string, position json.RawMessage) error {
	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO reading_progress (user_id, file_id, position, last_read_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (user_id, file_id) DO UPDATE
		SET position = $3, last_read_at = NOW()`,
		userID, fileID, []byte(position))
	return err
}

func (r *ReadingRepository) GetBookmarks(ctx context.Context, userID, fileID string) ([]*domain.FileBookmark, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, user_id, file_id, position, note, created_at
		 FROM file_bookmarks WHERE user_id = $1 AND file_id = $2
		 ORDER BY created_at DESC`,
		userID, fileID)
	if err != nil {
		return nil, fmt.Errorf("get bookmarks: %w", err)
	}
	defer rows.Close()

	var out []*domain.FileBookmark
	for rows.Next() {
		b := &domain.FileBookmark{}
		var pos []byte
		if err := rows.Scan(&b.ID, &b.UserID, &b.FileID, &pos, &b.Note, &b.CreatedAt); err != nil {
			return nil, err
		}
		b.Position = json.RawMessage(pos)
		out = append(out, b)
	}
	return out, nil
}

func (r *ReadingRepository) CreateBookmark(ctx context.Context, b *domain.FileBookmark) error {
	return r.db.Pool.QueryRow(ctx,
		`INSERT INTO file_bookmarks (user_id, file_id, position, note)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, created_at`,
		b.UserID, b.FileID, []byte(b.Position), b.Note,
	).Scan(&b.ID, &b.CreatedAt)
}

func (r *ReadingRepository) DeleteBookmark(ctx context.Context, id, userID string) error {
	tag, err := r.db.Pool.Exec(ctx,
		`DELETE FROM file_bookmarks WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrFileNotFound
	}
	return nil
}

func (r *ReadingRepository) GetReadingStatus(ctx context.Context, userID, fileID string) (*domain.ReadingStatus, error) {
	s := &domain.ReadingStatus{}
	err := r.db.Pool.QueryRow(ctx,
		`SELECT user_id, file_id, status, updated_at
		 FROM reading_status WHERE user_id = $1 AND file_id = $2`,
		userID, fileID,
	).Scan(&s.UserID, &s.FileID, &s.Status, &s.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get reading status: %w", err)
	}
	return s, nil
}

func (r *ReadingRepository) UpsertReadingStatus(ctx context.Context, userID, fileID, status string) error {
	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO reading_status (user_id, file_id, status, updated_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (user_id, file_id) DO UPDATE
		SET status = $3, updated_at = NOW()`,
		userID, fileID, status)
	return err
}

func (r *ReadingRepository) DeleteReadingStatus(ctx context.Context, userID, fileID string) error {
	_, err := r.db.Pool.Exec(ctx,
		`DELETE FROM reading_status WHERE user_id = $1 AND file_id = $2`, userID, fileID)
	return err
}

// GetUserReadingList возвращает файлы с заданным статусом чтения для юзера.
func (r *ReadingRepository) GetUserReadingList(ctx context.Context, userID, status, cursor string, limit int) ([]*domain.File, string, error) {
	if limit <= 0 {
		limit = 20
	}
	args := []interface{}{userID, status}
	n := 3
	cursorClause := ""
	if cursor != "" {
		cursorClause = fmt.Sprintf(`AND f.created_at < (SELECT created_at FROM files WHERE id = $%d)`, n)
		args = append(args, cursor)
		n++
	}

	query := fmt.Sprintf(`
		SELECT f.id, f.user_id, f.filename,
		       COALESCE(f.title, f.filename), COALESCE(f.author_name, ''), COALESCE(f.language, ''),
		       f.file_url, f.mime_type, f.file_size,
		       COALESCE(f.category_id::text, ''), f.downloads_count, f.likes_count, f.is_previewable,
		       COALESCE(f.preview_url, ''), COALESCE(f.description, ''),
		       COALESCE(f.pages_count, 0), COALESCE(f.doc_format, ''),
		       f.created_at,
		       u.id, u.username, u.full_name, u.avatar_url, u.is_verified
		FROM files f
		JOIN users u ON u.id = f.user_id
		JOIN reading_status rs ON rs.file_id = f.id AND rs.user_id = $1
		WHERE rs.status = $2 %s
		ORDER BY rs.updated_at DESC, f.created_at DESC
		LIMIT $%d`, cursorClause, n)
	args = append(args, limit+1)

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("reading list: %w", err)
	}
	defer rows.Close()

	files, err := scanFiles(rows)
	if err != nil {
		return nil, "", err
	}

	nextCursor := ""
	if len(files) > limit {
		nextCursor = files[limit-1].ID
		files = files[:limit]
	}
	return files, nextCursor, nil
}
