package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/seeu/backend/internal/domain"
)

type CollectionRepository struct {
	db *DB
}

func NewCollectionRepository(db *DB) *CollectionRepository {
	return &CollectionRepository{db: db}
}

func (r *CollectionRepository) GetUserCollections(ctx context.Context, userID string) ([]*domain.Collection, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT c.id, c.user_id, c.name, c.description,
		       COALESCE(c.cover_file_id::text, ''),
		       COUNT(cf.file_id) AS files_count,
		       c.created_at, c.updated_at
		FROM collections c
		LEFT JOIN collection_files cf ON cf.collection_id = c.id
		WHERE c.user_id = $1
		GROUP BY c.id
		ORDER BY c.updated_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("get collections: %w", err)
	}
	defer rows.Close()

	var out []*domain.Collection
	for rows.Next() {
		c := &domain.Collection{}
		if err := rows.Scan(&c.ID, &c.UserID, &c.Name, &c.Description,
			&c.CoverFileID, &c.FilesCount, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

func (r *CollectionRepository) GetCollectionByID(ctx context.Context, id, userID string) (*domain.Collection, error) {
	c := &domain.Collection{}
	err := r.db.Pool.QueryRow(ctx, `
		SELECT c.id, c.user_id, c.name, c.description,
		       COALESCE(c.cover_file_id::text, ''),
		       COUNT(cf.file_id) AS files_count,
		       c.created_at, c.updated_at
		FROM collections c
		LEFT JOIN collection_files cf ON cf.collection_id = c.id
		WHERE c.id = $1 AND c.user_id = $2
		GROUP BY c.id`, id, userID,
	).Scan(&c.ID, &c.UserID, &c.Name, &c.Description,
		&c.CoverFileID, &c.FilesCount, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get collection: %w", err)
	}

	// Load files
	rows, err := r.db.Pool.Query(ctx, `
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
		JOIN collection_files cf ON cf.file_id = f.id
		WHERE cf.collection_id = $1
		ORDER BY cf.added_at DESC`, id)
	if err != nil {
		return nil, fmt.Errorf("collection files: %w", err)
	}
	defer rows.Close()

	files, err := scanFiles(rows)
	if err != nil {
		return nil, err
	}
	c.Files = files
	return c, nil
}

func (r *CollectionRepository) CreateCollection(ctx context.Context, c *domain.Collection) error {
	return r.db.Pool.QueryRow(ctx, `
		INSERT INTO collections (user_id, name, description, cover_file_id)
		VALUES ($1, $2, $3, NULLIF($4, '')::uuid)
		RETURNING id, created_at, updated_at`,
		c.UserID, c.Name, c.Description, c.CoverFileID,
	).Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt)
}

func (r *CollectionRepository) UpdateCollection(ctx context.Context, c *domain.Collection) error {
	tag, err := r.db.Pool.Exec(ctx, `
		UPDATE collections
		SET name = $1, description = $2, cover_file_id = NULLIF($3, '')::uuid, updated_at = NOW()
		WHERE id = $4 AND user_id = $5`,
		c.Name, c.Description, c.CoverFileID, c.ID, c.UserID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrFileNotFound
	}
	return nil
}

func (r *CollectionRepository) DeleteCollection(ctx context.Context, id, userID string) error {
	tag, err := r.db.Pool.Exec(ctx,
		`DELETE FROM collections WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrFileNotFound
	}
	return nil
}

func (r *CollectionRepository) AddFile(ctx context.Context, collectionID, fileID, userID string) error {
	// verify ownership first
	var exists bool
	if err := r.db.Pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM collections WHERE id = $1 AND user_id = $2)`,
		collectionID, userID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return domain.ErrFileNotFound
	}
	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO collection_files (collection_id, file_id)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING`, collectionID, fileID)
	return err
}

func (r *CollectionRepository) RemoveFile(ctx context.Context, collectionID, fileID, userID string) error {
	// verify ownership
	var owns bool
	if err := r.db.Pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM collections WHERE id = $1 AND user_id = $2)`,
		collectionID, userID).Scan(&owns); err != nil {
		return err
	}
	if !owns {
		return domain.ErrFileNotFound
	}
	_, err := r.db.Pool.Exec(ctx,
		`DELETE FROM collection_files WHERE collection_id = $1 AND file_id = $2`,
		collectionID, fileID)
	return err
}

// GetFileStats возвращает статистику файла (только для owner).
func (r *CollectionRepository) GetFileStats(ctx context.Context, fileID string) (map[string]int, error) {
	var downloads, likes, readers, currentlyReading int
	err := r.db.Pool.QueryRow(ctx, `
		SELECT
			f.downloads_count,
			f.likes_count,
			COUNT(DISTINCT rs.user_id),
			COUNT(DISTINCT CASE WHEN rs.status = 'reading' THEN rs.user_id END)
		FROM files f
		LEFT JOIN reading_status rs ON rs.file_id = f.id
		WHERE f.id = $1
		GROUP BY f.downloads_count, f.likes_count`, fileID,
	).Scan(&downloads, &likes, &readers, &currentlyReading)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrFileNotFound
		}
		return nil, fmt.Errorf("file stats: %w", err)
	}
	return map[string]int{
		"downloads_count":   downloads,
		"likes_count":       likes,
		"readers_count":     readers,
		"currently_reading": currentlyReading,
	}, nil
}
