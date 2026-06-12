package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/seeu/backend/internal/domain"
)

type FileRepository struct {
	db *DB
}

func NewFileRepository(db *DB) *FileRepository {
	return &FileRepository{db: db}
}

// CheckUserVisibility — privacy-check для library service (тот же паттерн
// что в video_repo.CheckUserVisibility). Cross-service shared DB.
func (r *FileRepository) CheckUserVisibility(ctx context.Context, ownerID, viewerID string) error {
	var isPrivate, isFollower bool
	err := r.db.Pool.QueryRow(ctx, `
		SELECT
			u.is_private,
			COALESCE((
				SELECT TRUE FROM follows
				WHERE follower_id = $2 AND following_id = u.id
				LIMIT 1
			), FALSE)
		FROM users u WHERE u.id = $1`,
		ownerID, viewerID,
	).Scan(&isPrivate, &isFollower)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrUserNotFound
		}
		return fmt.Errorf("check visibility: %w", err)
	}
	if isPrivate && ownerID != viewerID && !isFollower {
		return domain.ErrPrivateAccount
	}
	return nil
}

func (r *FileRepository) Create(ctx context.Context, f *domain.File) error {
	// Все форматы библиотеки v2 поддерживают inline-просмотр
	previewable := isPreviewable(f.DocFormat)

	query := `
		INSERT INTO files (
			user_id, filename, title, author_name, language,
			file_url, mime_type, file_size, category_id,
			is_previewable, description, pages_count, doc_format, extracted_text
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, NULLIF($9, '')::uuid,
			$10, $11, $12, $13, NULLIF($14, '')
		)
		RETURNING id, downloads_count, created_at`

	return r.db.Pool.QueryRow(ctx, query,
		f.UserID, f.Filename, f.Title, f.AuthorName, f.Language,
		f.FileURL, f.MimeType, f.FileSize, f.CategoryID,
		previewable, f.Description, f.PagesCount, f.DocFormat, f.ExtractedText,
	).Scan(&f.ID, &f.DownloadsCount, &f.CreatedAt)
}

func (r *FileRepository) GetByID(ctx context.Context, id string) (*domain.File, error) {
	query := `
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
		WHERE f.id = $1`

	file := &domain.File{User: &domain.UserShort{}}
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&file.ID, &file.UserID, &file.Filename,
		&file.Title, &file.AuthorName, &file.Language,
		&file.FileURL, &file.MimeType, &file.FileSize,
		&file.CategoryID, &file.DownloadsCount, &file.LikesCount, &file.IsPreviewable,
		&file.PreviewURL, &file.Description,
		&file.PagesCount, &file.DocFormat,
		&file.CreatedAt,
		&file.User.ID, &file.User.Username, &file.User.FullName,
		&file.User.AvatarURL, &file.User.IsVerified,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrFileNotFound
		}
		return nil, fmt.Errorf("get file: %w", err)
	}
	return file, nil
}

// GetExtractedText возвращает только extracted_text файла (не грузим в обычных запросах).
func (r *FileRepository) GetExtractedText(ctx context.Context, id string) (string, error) {
	var text *string
	err := r.db.Pool.QueryRow(ctx,
		`SELECT extracted_text FROM files WHERE id = $1`, id,
	).Scan(&text)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", domain.ErrFileNotFound
		}
		return "", fmt.Errorf("get extracted text: %w", err)
	}
	if text == nil {
		return "", nil
	}
	return *text, nil
}

// Trending (LIB-6) — top files за последние 7 дней по hot-score.
func (r *FileRepository) Trending(ctx context.Context, limit int) ([]*domain.File, error) {
	if limit <= 0 {
		limit = 10
	}
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
		WHERE f.created_at > NOW() - INTERVAL '7 days'
		ORDER BY (f.likes_count * 2 + f.downloads_count) DESC, f.created_at DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("trending files: %w", err)
	}
	defer rows.Close()
	return scanFiles(rows)
}

func (r *FileRepository) List(ctx context.Context, categoryID string, limit, offset int) ([]*domain.File, int, error) {
	const selectCols = `
		SELECT f.id, f.user_id, f.filename,
		       COALESCE(f.title, f.filename), COALESCE(f.author_name, ''), COALESCE(f.language, ''),
		       f.file_url, f.mime_type, f.file_size,
		       COALESCE(f.category_id::text, ''), f.downloads_count, f.likes_count, f.is_previewable,
		       COALESCE(f.preview_url, ''), COALESCE(f.description, ''),
		       COALESCE(f.pages_count, 0), COALESCE(f.doc_format, ''),
		       f.created_at,
		       u.id, u.username, u.full_name, u.avatar_url, u.is_verified
		FROM files f
		JOIN users u ON u.id = f.user_id`

	countQuery := `SELECT COUNT(*) FROM files`
	listQuery := selectCols

	args := []interface{}{}
	argIdx := 1

	if categoryID != "" {
		countQuery += fmt.Sprintf(" WHERE category_id = $%d", argIdx)
		listQuery += fmt.Sprintf(" WHERE f.category_id = $%d", argIdx)
		args = append(args, categoryID)
		argIdx++
	}

	var total int
	if err := r.db.Pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	listQuery += fmt.Sprintf(" ORDER BY f.created_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := r.db.Pool.Query(ctx, listQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	files, err := scanFiles(rows)
	return files, total, err
}

func (r *FileRepository) GetUserFiles(ctx context.Context, userID string, limit, offset int) ([]*domain.File, int, error) {
	var total int
	if err := r.db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM files WHERE user_id = $1`, userID).Scan(&total); err != nil {
		return nil, 0, err
	}

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
		WHERE f.user_id = $1
		ORDER BY f.created_at DESC LIMIT $2 OFFSET $3`,
		userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	files, err := scanFiles(rows)
	return files, total, err
}

func (r *FileRepository) Delete(ctx context.Context, id, userID string) error {
	result, err := r.db.Pool.Exec(ctx, `DELETE FROM files WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return domain.ErrFileNotFound
	}
	return nil
}

func (r *FileRepository) RecordDownload(ctx context.Context, fileID, userID string) error {
	_, err := r.db.Pool.Exec(ctx, `INSERT INTO file_downloads (file_id, user_id) VALUES ($1, $2)`, fileID, userID)
	if err != nil {
		return err
	}
	_, err = r.db.Pool.Exec(ctx, `UPDATE files SET downloads_count = downloads_count + 1 WHERE id = $1`, fileID)
	return err
}

// LikeFile — записывает в polymorphic `likes` (entity_type='file') + инкремент
// files.likes_count. Возвращает true если лайк новый (не было ранее), false
// если идемпотентный repeat-вызов.
func (r *FileRepository) LikeFile(ctx context.Context, fileID, userID string) (bool, error) {
	tag, err := r.db.Pool.Exec(ctx, `
		INSERT INTO likes (user_id, entity_id, entity_type)
		VALUES ($1, $2, 'file')
		ON CONFLICT (user_id, entity_id, entity_type) DO NOTHING`,
		userID, fileID)
	if err != nil {
		return false, fmt.Errorf("insert like: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return false, nil // уже лайкнул раньше
	}
	if _, err := r.db.Pool.Exec(ctx,
		`UPDATE files SET likes_count = likes_count + 1 WHERE id = $1`, fileID); err != nil {
		return false, fmt.Errorf("inc likes_count: %w", err)
	}
	return true, nil
}

func (r *FileRepository) UnlikeFile(ctx context.Context, fileID, userID string) (bool, error) {
	tag, err := r.db.Pool.Exec(ctx,
		`DELETE FROM likes WHERE user_id = $1 AND entity_id = $2 AND entity_type = 'file'`,
		userID, fileID)
	if err != nil {
		return false, fmt.Errorf("delete like: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return false, nil
	}
	if _, err := r.db.Pool.Exec(ctx,
		`UPDATE files SET likes_count = GREATEST(likes_count - 1, 0) WHERE id = $1`, fileID); err != nil {
		return false, fmt.Errorf("dec likes_count: %w", err)
	}
	return true, nil
}

func (r *FileRepository) IsFileLiked(ctx context.Context, fileID, userID string) (bool, error) {
	var exists bool
	err := r.db.Pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM likes
			WHERE user_id = $1 AND entity_id = $2 AND entity_type = 'file'
		)`, userID, fileID).Scan(&exists)
	return exists, err
}

func (r *FileRepository) GetCategories(ctx context.Context) ([]*domain.FileCategory, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, name, COALESCE(slug, ''), COALESCE(sort_order, 0), created_at
		FROM file_categories
		ORDER BY COALESCE(sort_order, 999), name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cats []*domain.FileCategory
	for rows.Next() {
		c := &domain.FileCategory{}
		if err := rows.Scan(&c.ID, &c.Name, &c.Slug, &c.SortOrder, &c.CreatedAt); err != nil {
			return nil, err
		}
		cats = append(cats, c)
	}
	return cats, nil
}

// isPreviewable возвращает true для всех форматов библиотеки v2 —
// каждый поддерживает inline-просмотр в Flutter-ридере.
func isPreviewable(docFormat string) bool {
	switch docFormat {
	case "pdf", "epub", "fb2", "docx", "pptx", "txt", "rtf", "md", "odt", "odp":
		return true
	}
	// Legacy: поддержка старых записей по MIME
	return false
}

// scanFiles сканирует rows с расширенными полями v2 в []*domain.File.
// Все запросы List/Trending/GetUserFiles должны использовать эту функцию
// для единообразия порядка колонок.
func scanFiles(rows interface {
	Next() bool
	Scan(dest ...any) error
	Close()
}) ([]*domain.File, error) {
	defer rows.Close()
	var files []*domain.File
	for rows.Next() {
		f := &domain.File{User: &domain.UserShort{}}
		if err := rows.Scan(
			&f.ID, &f.UserID, &f.Filename,
			&f.Title, &f.AuthorName, &f.Language,
			&f.FileURL, &f.MimeType, &f.FileSize,
			&f.CategoryID, &f.DownloadsCount, &f.LikesCount, &f.IsPreviewable,
			&f.PreviewURL, &f.Description,
			&f.PagesCount, &f.DocFormat,
			&f.CreatedAt,
			&f.User.ID, &f.User.Username, &f.User.FullName,
			&f.User.AvatarURL, &f.User.IsVerified,
		); err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, nil
}
