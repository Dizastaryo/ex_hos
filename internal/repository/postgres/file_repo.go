package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

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
			is_previewable, description, pages_count, doc_format, extracted_text, cover_url
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, NULLIF($9, '')::uuid,
			$10, $11, $12, $13, NULLIF($14, ''), $15
		)
		RETURNING id, downloads_count, created_at`

	return r.db.Pool.QueryRow(ctx, query,
		f.UserID, f.Filename, f.Title, f.AuthorName, f.Language,
		f.FileURL, f.MimeType, f.FileSize, f.CategoryID,
		previewable, f.Description, f.PagesCount, f.DocFormat, f.ExtractedText, f.CoverURL,
	).Scan(&f.ID, &f.DownloadsCount, &f.CreatedAt)
}

func (r *FileRepository) GetByID(ctx context.Context, id string) (*domain.File, error) {
	query := `
		SELECT f.id, f.user_id, f.filename,
		       COALESCE(f.title, f.filename), COALESCE(f.author_name, ''), COALESCE(f.language, ''),
		       f.file_url, f.mime_type, f.file_size,
		       COALESCE(f.category_id::text, ''), f.downloads_count, f.likes_count, COALESCE(f.views_count, 0), COALESCE(f.ratings_count, 0), COALESCE(f.ratings_sum, 0), f.is_previewable,
		       COALESCE(f.preview_url, ''), COALESCE(f.cover_url, ''), COALESCE(f.description, ''),
		       COALESCE(f.pages_count, 0), COALESCE(f.doc_format, ''),
		       COALESCE(f.pdf_conversion_status, 'none'),
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
		&file.CategoryID, &file.DownloadsCount, &file.LikesCount, &file.ViewsCount, &file.RatingsCount, &file.RatingsSum, &file.IsPreviewable,
		&file.PreviewURL, &file.CoverURL, &file.Description,
		&file.PagesCount, &file.DocFormat,
		&file.PdfConversionStatus,
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

// UpdateExtractedText saves newly extracted text for an existing file.
func (r *FileRepository) UpdateExtractedText(ctx context.Context, id, text string) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE files SET extracted_text = $1 WHERE id = $2`, text, id,
	)
	return err
}

// GetPdfCacheURL возвращает закэшированный URL PDF-версии файла.
// Возвращает ("", nil) если кэша ещё нет.
func (r *FileRepository) GetPdfCacheURL(ctx context.Context, id string) (string, error) {
	var url *string
	err := r.db.Pool.QueryRow(ctx,
		`SELECT pdf_cache_url FROM files WHERE id = $1`, id,
	).Scan(&url)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", domain.ErrFileNotFound
		}
		return "", fmt.Errorf("get pdf cache url: %w", err)
	}
	if url == nil {
		return "", nil
	}
	return *url, nil
}

// SetPdfCacheURL сохраняет URL PDF-версии после успешной конвертации.
func (r *FileRepository) SetPdfCacheURL(ctx context.Context, id, url string) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE files SET pdf_cache_url = $1 WHERE id = $2`, url, id,
	)
	return err
}

// GetPdfConversionStatus returns the current pdf_conversion_status for a file.
func (r *FileRepository) GetPdfConversionStatus(ctx context.Context, id string) (string, error) {
	var status string
	err := r.db.Pool.QueryRow(ctx,
		`SELECT COALESCE(pdf_conversion_status, 'none') FROM files WHERE id = $1`, id,
	).Scan(&status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", domain.ErrFileNotFound
		}
		return "", fmt.Errorf("get pdf conversion status: %w", err)
	}
	return status, nil
}

// SetPdfConversionStatus updates pdf_conversion_status for a file.
func (r *FileRepository) SetPdfConversionStatus(ctx context.Context, id, status string) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE files SET pdf_conversion_status = $1 WHERE id = $2`, status, id,
	)
	return err
}

// TryClaimConversion atomically transitions pdf_conversion_status 'pending' → 'converting'.
// Returns true if this caller won the race and should proceed with conversion.
func (r *FileRepository) TryClaimConversion(ctx context.Context, id string) (bool, error) {
	tag, err := r.db.Pool.Exec(ctx,
		`UPDATE files SET pdf_conversion_status = 'converting'
		 WHERE id = $1 AND pdf_conversion_status = 'pending'`, id,
	)
	if err != nil {
		return false, fmt.Errorf("claim conversion: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// GetPendingConversions returns files queued for background PDF conversion (up to 200).
func (r *FileRepository) GetPendingConversions(ctx context.Context) ([]*domain.File, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, file_url, doc_format
		FROM files
		WHERE pdf_conversion_status = 'pending'
		ORDER BY created_at ASC
		LIMIT 200`)
	if err != nil {
		return nil, fmt.Errorf("get pending conversions: %w", err)
	}
	defer rows.Close()
	var files []*domain.File
	for rows.Next() {
		f := &domain.File{}
		if err := rows.Scan(&f.ID, &f.FileURL, &f.DocFormat); err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, nil
}

// Trending (LIB-6) — top files по hot-score за указанный период.
// period: "week" (7d, default) | "month" (30d) | "all"
func (r *FileRepository) Trending(ctx context.Context, limit int, period string) ([]*domain.File, error) {
	if limit <= 0 {
		limit = 10
	}
	var intervalClause string
	switch period {
	case "month":
		intervalClause = "AND f.created_at > NOW() - INTERVAL '30 days'"
	case "all":
		intervalClause = ""
	default: // "week"
		intervalClause = "AND f.created_at > NOW() - INTERVAL '7 days'"
	}
	query := fmt.Sprintf(`
		SELECT f.id, f.user_id, f.filename,
		       COALESCE(f.title, f.filename), COALESCE(f.author_name, ''), COALESCE(f.language, ''),
		       f.file_url, f.mime_type, f.file_size,
		       COALESCE(f.category_id::text, ''), f.downloads_count, f.likes_count, COALESCE(f.views_count, 0), COALESCE(f.ratings_count, 0), COALESCE(f.ratings_sum, 0), f.is_previewable,
		       COALESCE(f.preview_url, ''), COALESCE(f.cover_url, ''), COALESCE(f.description, ''),
		       COALESCE(f.pages_count, 0), COALESCE(f.doc_format, ''),
		       COALESCE(f.pdf_conversion_status, 'none'),
		       f.created_at,
		       u.id, u.username, u.full_name, u.avatar_url, u.is_verified
		FROM files f
		JOIN users u ON u.id = f.user_id
		WHERE 1=1 %s
		ORDER BY (
			f.likes_count * 3
			+ f.downloads_count * 2
			+ COALESCE(f.views_count, 0)
			+ CASE WHEN COALESCE(f.ratings_count, 0) > 0
				THEN ROUND((COALESCE(f.ratings_sum, 0)::float / f.ratings_count) * 10)
				ELSE 0 END
		) DESC, f.created_at DESC
		LIMIT $1`, intervalClause)
	rows, err := r.db.Pool.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("trending files: %w", err)
	}
	defer rows.Close()
	return scanFiles(rows)
}

// List возвращает файлы с фильтрацией по категории, полнотекстовым поиском,
// сортировкой и cursor-based пагинацией.
// Возвращает []*domain.File и nextCursor (пустая строка — это последняя страница).
func (r *FileRepository) List(ctx context.Context, p domain.FileListParams) ([]*domain.File, string, error) {
	const selectCols = `
		SELECT f.id, f.user_id, f.filename,
		       COALESCE(f.title, f.filename), COALESCE(f.author_name, ''), COALESCE(f.language, ''),
		       f.file_url, f.mime_type, f.file_size,
		       COALESCE(f.category_id::text, ''), f.downloads_count, f.likes_count, COALESCE(f.views_count, 0), COALESCE(f.ratings_count, 0), COALESCE(f.ratings_sum, 0), f.is_previewable,
		       COALESCE(f.preview_url, ''), COALESCE(f.cover_url, ''), COALESCE(f.description, ''),
		       COALESCE(f.pages_count, 0), COALESCE(f.doc_format, ''),
		       COALESCE(f.pdf_conversion_status, 'none'),
		       f.created_at,
		       u.id, u.username, u.full_name, u.avatar_url, u.is_verified
		FROM files f
		JOIN users u ON u.id = f.user_id`

	args := []interface{}{}
	n := 1 // next placeholder index
	where := []string{}

	// Фильтр по категории
	if p.CategoryID != "" {
		where = append(where, fmt.Sprintf("f.category_id = $%d", n))
		args = append(args, p.CategoryID)
		n++
	}

	isSearch := p.Q != "" || p.AuthorName != ""

	// Поиск через tsvector (по названию/описанию) + ILIKE fallback.
	// tsvector использует 'simple' конфиг (см. migration 000081), поэтому
	// plainto_tsquery тоже 'simple'. ILIKE fallback ловит частичные совпадения.
	qArgPos := 0
	if p.Q != "" {
		qArgPos = n
		where = append(where, fmt.Sprintf(
			"(f.search_vector @@ plainto_tsquery('simple', $%d) OR f.title ILIKE '%%' || $%d || '%%' OR f.author_name ILIKE '%%' || $%d || '%%')",
			n, n, n,
		))
		args = append(args, p.Q)
		n++
	}

	// Поиск по автору (ILIKE)
	if p.AuthorName != "" {
		where = append(where, fmt.Sprintf("f.author_name ILIKE '%%' || $%d || '%%'", n))
		args = append(args, p.AuthorName)
		n++
	}

	// Фильтр по формату документа
	if p.DocFormat != "" {
		where = append(where, fmt.Sprintf("f.doc_format = $%d", n))
		args = append(args, p.DocFormat)
		n++
	}

	// Фильтр по языку
	if p.Language != "" {
		where = append(where, fmt.Sprintf("f.language = $%d", n))
		args = append(args, p.Language)
		n++
	}

	// Исключить конкретный файл (для "похожих файлов")
	if p.ExcludeID != "" {
		where = append(where, fmt.Sprintf("f.id != $%d", n))
		args = append(args, p.ExcludeID)
		n++
	}

	// Cursor (только при не-поисковой выборке и sort=date)
	if p.Cursor != "" && !isSearch && (p.Sort == "" || p.Sort == "date") {
		where = append(where, fmt.Sprintf("f.created_at < (SELECT created_at FROM files WHERE id = $%d)", n))
		args = append(args, p.Cursor)
		n++
	}

	query := selectCols
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}

	// ORDER BY
	if isSearch {
		if p.Q != "" && qArgPos > 0 {
			// Rank by tsvector rank (when matched), then recency
			query += fmt.Sprintf(
				" ORDER BY ts_rank(f.search_vector, plainto_tsquery('simple', $%d)) DESC, f.created_at DESC",
				qArgPos,
			)
		} else {
			query += " ORDER BY f.created_at DESC"
		}
	} else {
		switch p.Sort {
		case "likes":
			query += " ORDER BY f.likes_count DESC, f.created_at DESC"
		case "downloads":
			query += " ORDER BY f.downloads_count DESC, f.created_at DESC"
		case "views":
			query += " ORDER BY f.views_count DESC, f.created_at DESC"
		case "rating":
			query += " ORDER BY CASE WHEN f.ratings_count = 0 THEN 0 ELSE f.ratings_sum::float / f.ratings_count END DESC, f.ratings_count DESC, f.created_at DESC"
		case "title":
			query += " ORDER BY f.title ASC"
		default: // "date"
			query += " ORDER BY f.created_at DESC"
		}
	}

	// Fetch limit+1 чтобы знать есть ли следующая страница
	limit := p.Limit
	if limit <= 0 {
		limit = 20
	}
	query += fmt.Sprintf(" LIMIT $%d", n)
	args = append(args, limit+1)

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("list files: %w", err)
	}
	defer rows.Close()

	files, err := scanFiles(rows)
	if err != nil {
		return nil, "", err
	}

	nextCursor := ""
	if len(files) > limit {
		nextCursor = files[limit].ID
		files = files[:limit]
	}
	return files, nextCursor, nil
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
		       COALESCE(f.category_id::text, ''), f.downloads_count, f.likes_count, COALESCE(f.views_count, 0), COALESCE(f.ratings_count, 0), COALESCE(f.ratings_sum, 0), f.is_previewable,
		       COALESCE(f.preview_url, ''), COALESCE(f.cover_url, ''), COALESCE(f.description, ''),
		       COALESCE(f.pages_count, 0), COALESCE(f.doc_format, ''),
		       COALESCE(f.pdf_conversion_status, 'none'),
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

func (r *FileRepository) UpdateMeta(ctx context.Context, fileID, userID string, req domain.UpdateFileMetaRequest) error {
	result, err := r.db.Pool.Exec(ctx, `
		UPDATE files
		SET title       = CASE WHEN $3 != '' THEN $3 ELSE title END,
		    author_name = CASE WHEN $4 != '' THEN $4 ELSE author_name END,
		    description = $5,
		    category_id = NULLIF($6, '')::uuid,
		    language    = CASE WHEN $7 != '' THEN $7 ELSE language END
		WHERE id = $1 AND user_id = $2`,
		fileID, userID, req.Title, req.AuthorName, req.Description, req.CategoryID, req.Language,
	)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return domain.ErrFileNotFound
	}
	return nil
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
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `INSERT INTO file_downloads (file_id, user_id) VALUES ($1, $2)`, fileID, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE files SET downloads_count = downloads_count + 1 WHERE id = $1`, fileID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// LikeFile — записывает в polymorphic `likes` (entity_type='file') + инкремент
// files.likes_count. Возвращает true если лайк новый (не было ранее), false
// если идемпотентный repeat-вызов.
func (r *FileRepository) LikeFile(ctx context.Context, fileID, userID string) (bool, error) {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `
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
	if _, err := tx.Exec(ctx,
		`UPDATE files SET likes_count = likes_count + 1 WHERE id = $1`, fileID); err != nil {
		return false, fmt.Errorf("inc likes_count: %w", err)
	}
	return true, tx.Commit(ctx)
}

func (r *FileRepository) UnlikeFile(ctx context.Context, fileID, userID string) (bool, error) {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx,
		`DELETE FROM likes WHERE user_id = $1 AND entity_id = $2 AND entity_type = 'file'`,
		userID, fileID)
	if err != nil {
		return false, fmt.Errorf("delete like: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return false, nil
	}
	if _, err := tx.Exec(ctx,
		`UPDATE files SET likes_count = GREATEST(likes_count - 1, 0) WHERE id = $1`, fileID); err != nil {
		return false, fmt.Errorf("dec likes_count: %w", err)
	}
	return true, tx.Commit(ctx)
}

// RateFile upserts a rating (1–5) for a file by the user, keeping aggregate columns updated.
func (r *FileRepository) RateFile(ctx context.Context, fileID, userID string, rating int, reviewText string) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Check for existing rating to adjust the delta
	var prevRating int
	err = tx.QueryRow(ctx,
		`SELECT rating FROM file_ratings WHERE user_id = $1 AND file_id = $2`,
		userID, fileID).Scan(&prevRating)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("check prev rating: %w", err)
	}

	// Upsert rating row (with review_text)
	_, err = tx.Exec(ctx, `
		INSERT INTO file_ratings (user_id, file_id, rating, review_text)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, file_id) DO UPDATE SET rating = $3, review_text = $4, updated_at = NOW()`,
		userID, fileID, rating, reviewText)
	if err != nil {
		return fmt.Errorf("upsert rating: %w", err)
	}

	// Update file aggregate counters
	if prevRating == 0 {
		// New rating
		_, err = tx.Exec(ctx,
			`UPDATE files SET ratings_count = ratings_count + 1, ratings_sum = ratings_sum + $2 WHERE id = $1`,
			fileID, rating)
	} else {
		// Changed rating — adjust sum only
		delta := rating - prevRating
		_, err = tx.Exec(ctx,
			`UPDATE files SET ratings_sum = ratings_sum + $2 WHERE id = $1`,
			fileID, delta)
	}
	if err != nil {
		return fmt.Errorf("update file rating agg: %w", err)
	}
	return tx.Commit(ctx)
}

// GetUserRating returns the user's rating and review for a file (0/"" if not rated).
func (r *FileRepository) GetUserRating(ctx context.Context, fileID, userID string) (int, string, error) {
	var rating int
	var reviewText string
	err := r.db.Pool.QueryRow(ctx,
		`SELECT rating, COALESCE(review_text, '') FROM file_ratings WHERE user_id = $1 AND file_id = $2`,
		userID, fileID).Scan(&rating, &reviewText)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, "", nil
		}
		return 0, "", err
	}
	return rating, reviewText, nil
}

// GetFileReviews returns recent reviews for a file with user info.
func (r *FileRepository) GetFileReviews(ctx context.Context, fileID string, limit int) ([]map[string]interface{}, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := r.db.Pool.Query(ctx, `
		SELECT fr.user_id, u.username, u.full_name, COALESCE(u.avatar_url, ''),
		       fr.rating, COALESCE(fr.review_text, ''), fr.updated_at
		FROM file_ratings fr
		JOIN users u ON u.id = fr.user_id
		WHERE fr.file_id = $1 AND fr.review_text <> ''
		ORDER BY fr.updated_at DESC
		LIMIT $2`, fileID, limit)
	if err != nil {
		return nil, fmt.Errorf("get file reviews: %w", err)
	}
	defer rows.Close()

	var result []map[string]interface{}
	for rows.Next() {
		var userID, username, fullName, avatarURL, reviewText string
		var rating int
		var updatedAt interface{}
		if err := rows.Scan(&userID, &username, &fullName, &avatarURL, &rating, &reviewText, &updatedAt); err != nil {
			return nil, err
		}
		result = append(result, map[string]interface{}{
			"user_id":     userID,
			"username":    username,
			"full_name":   fullName,
			"avatar_url":  avatarURL,
			"rating":      rating,
			"review_text": reviewText,
			"updated_at":  updatedAt,
		})
	}
	if result == nil {
		result = []map[string]interface{}{}
	}
	return result, rows.Err()
}

// GetUserRatingBatch returns a map of fileID→rating for files the user has rated.
func (r *FileRepository) GetUserRatingBatch(ctx context.Context, userID string, fileIDs []string) (map[string]int, error) {
	if len(fileIDs) == 0 {
		return nil, nil
	}
	rows, err := r.db.Pool.Query(ctx,
		`SELECT file_id, rating FROM file_ratings WHERE user_id = $1 AND file_id = ANY($2)`,
		userID, fileIDs)
	if err != nil {
		return nil, fmt.Errorf("batch user ratings: %w", err)
	}
	defer rows.Close()
	result := make(map[string]int, len(fileIDs))
	for rows.Next() {
		var fid string
		var rating int
		if err := rows.Scan(&fid, &rating); err != nil {
			return nil, err
		}
		result[fid] = rating
	}
	return result, rows.Err()
}

// IncrementViews atomically increments views_count for the given file.
// If userID is non-empty, also upserts a record in file_views (for recently-viewed history).
// For anonymous viewers only the counter is incremented.
func (r *FileRepository) IncrementViews(ctx context.Context, fileID, userID string) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE files SET views_count = views_count + 1 WHERE id = $1`, fileID)
	if err != nil {
		return err
	}
	if userID == "" {
		return nil
	}
	_, err = r.db.Pool.Exec(ctx, `
		INSERT INTO file_views (user_id, file_id, viewed_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (user_id, file_id) DO UPDATE SET viewed_at = NOW()`,
		userID, fileID)
	return err
}

// GetRecentlyViewed returns the last N files viewed by the user (newest first).
func (r *FileRepository) GetRecentlyViewed(ctx context.Context, userID string, limit int) ([]*domain.File, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.db.Pool.Query(ctx, `
		SELECT f.id, f.user_id, f.filename,
		       COALESCE(f.title, f.filename), COALESCE(f.author_name, ''), COALESCE(f.language, ''),
		       f.file_url, f.mime_type, f.file_size,
		       COALESCE(f.category_id::text, ''), f.downloads_count, f.likes_count, COALESCE(f.views_count, 0), COALESCE(f.ratings_count, 0), COALESCE(f.ratings_sum, 0), f.is_previewable,
		       COALESCE(f.preview_url, ''), COALESCE(f.cover_url, ''), COALESCE(f.description, ''),
		       COALESCE(f.pages_count, 0), COALESCE(f.doc_format, ''),
		       COALESCE(f.pdf_conversion_status, 'none'),
		       f.created_at,
		       u.id, u.username, u.full_name, u.avatar_url, u.is_verified
		FROM file_views fv
		JOIN files f ON f.id = fv.file_id
		JOIN users u ON u.id = f.user_id
		WHERE fv.user_id = $1
		ORDER BY fv.viewed_at DESC
		LIMIT $2`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("get recently viewed: %w", err)
	}
	return scanFiles(rows)
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

// IsFileLikedBatch returns a set of file IDs that the user has liked.
func (r *FileRepository) IsFileLikedBatch(ctx context.Context, userID string, fileIDs []string) (map[string]bool, error) {
	if len(fileIDs) == 0 {
		return nil, nil
	}
	rows, err := r.db.Pool.Query(ctx,
		`SELECT entity_id FROM likes
		 WHERE user_id = $1 AND entity_id = ANY($2) AND entity_type = 'file'`,
		userID, fileIDs)
	if err != nil {
		return nil, fmt.Errorf("batch file likes: %w", err)
	}
	defer rows.Close()
	result := make(map[string]bool, len(fileIDs))
	for rows.Next() {
		var fid string
		if err := rows.Scan(&fid); err != nil {
			return nil, err
		}
		result[fid] = true
	}
	return result, rows.Err()
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
			&f.CategoryID, &f.DownloadsCount, &f.LikesCount, &f.ViewsCount, &f.RatingsCount, &f.RatingsSum, &f.IsPreviewable,
			&f.PreviewURL, &f.CoverURL, &f.Description,
			&f.PagesCount, &f.DocFormat,
			&f.PdfConversionStatus,
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

// PopularAuthors возвращает авторов с наибольшим числом лайков+скачиваний.
func (r *FileRepository) PopularAuthors(ctx context.Context, limit int) ([]map[string]interface{}, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := r.db.Pool.Query(ctx, `
		SELECT author_name,
		       COUNT(*) AS files_count,
		       COALESCE(SUM(likes_count), 0) AS total_likes,
		       COALESCE(SUM(downloads_count), 0) AS total_downloads
		FROM files
		WHERE author_name IS NOT NULL AND author_name != ''
		GROUP BY author_name
		HAVING COUNT(*) >= 2
		ORDER BY (COALESCE(SUM(likes_count), 0) * 2 + COALESCE(SUM(downloads_count), 0)) DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("popular authors: %w", err)
	}
	defer rows.Close()

	var authors []map[string]interface{}
	for rows.Next() {
		var name string
		var filesCount, totalLikes, totalDownloads int
		if err := rows.Scan(&name, &filesCount, &totalLikes, &totalDownloads); err != nil {
			return nil, err
		}
		authors = append(authors, map[string]interface{}{
			"author_name":     name,
			"files_count":     filesCount,
			"total_likes":     totalLikes,
			"total_downloads": totalDownloads,
		})
	}
	return authors, rows.Err()
}

// FormatStats возвращает количество файлов по каждому формату.
func (r *FileRepository) FormatStats(ctx context.Context) ([]map[string]interface{}, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT COALESCE(doc_format, 'unknown') AS fmt, COUNT(*) AS cnt
		FROM files
		WHERE doc_format IS NOT NULL AND doc_format != ''
		GROUP BY doc_format
		ORDER BY cnt DESC`)
	if err != nil {
		return nil, fmt.Errorf("format stats: %w", err)
	}
	defer rows.Close()

	var stats []map[string]interface{}
	for rows.Next() {
		var format string
		var count int
		if err := rows.Scan(&format, &count); err != nil {
			return nil, err
		}
		stats = append(stats, map[string]interface{}{
			"format": format,
			"count":  count,
		})
	}
	return stats, rows.Err()
}

// RecommendedFiles returns personalised recommendations for the user:
//  1. Files from categories the user has interacted with (read/want/done)
//  2. Files by authors the user has read before
//  3. Scored by quality (likes × 3 + downloads × 2 + avg_rating × 10 + views)
//  4. Excludes files already in the user's reading history or status
func (r *FileRepository) RecommendedFiles(ctx context.Context, userID string, limit int) ([]*domain.File, error) {
	if limit <= 0 {
		limit = 10
	}
	query := `
		WITH user_excluded AS (
			SELECT file_id FROM reading_status WHERE user_id = $1
			UNION
			SELECT file_id FROM reading_progress WHERE user_id = $1
			UNION
			SELECT file_id FROM file_views WHERE user_id = $1
		),
		user_categories AS (
			SELECT DISTINCT f2.category_id
			FROM reading_status rs
			JOIN files f2 ON f2.id = rs.file_id
			WHERE rs.user_id = $1 AND f2.category_id IS NOT NULL
		),
		user_authors AS (
			SELECT DISTINCT f2.author_name
			FROM reading_status rs
			JOIN files f2 ON f2.id = rs.file_id
			WHERE rs.user_id = $1 AND f2.author_name IS NOT NULL AND f2.author_name != ''
		)
		SELECT f.id, f.user_id, f.filename,
		       COALESCE(f.title, f.filename), COALESCE(f.author_name, ''), COALESCE(f.language, ''),
		       f.file_url, f.mime_type, f.file_size,
		       COALESCE(f.category_id::text, ''), f.downloads_count, f.likes_count, COALESCE(f.views_count, 0), COALESCE(f.ratings_count, 0), COALESCE(f.ratings_sum, 0), f.is_previewable,
		       COALESCE(f.preview_url, ''), COALESCE(f.cover_url, ''), COALESCE(f.description, ''),
		       COALESCE(f.pages_count, 0), COALESCE(f.doc_format, ''),
		       COALESCE(f.pdf_conversion_status, 'none'),
		       f.created_at,
		       u.id, u.username, u.full_name, u.avatar_url, u.is_verified
		FROM files f
		JOIN users u ON u.id = f.user_id
		WHERE f.id NOT IN (SELECT file_id FROM user_excluded)
		  AND (
		      f.category_id IN (SELECT category_id FROM user_categories)
		      OR (f.author_name IS NOT NULL AND f.author_name IN (SELECT author_name FROM user_authors))
		  )
		ORDER BY (
			f.likes_count * 3
			+ f.downloads_count * 2
			+ COALESCE(f.views_count, 0)
			+ CASE WHEN COALESCE(f.ratings_count, 0) > 0
				THEN ROUND((COALESCE(f.ratings_sum, 0)::float / f.ratings_count) * 10)
				ELSE 0 END
		) DESC, f.created_at DESC
		LIMIT $2`
	rows, err := r.db.Pool.Query(ctx, query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("recommended files: %w", err)
	}
	defer rows.Close()
	return scanFiles(rows)
}

// GetSocialPicks returns files that users you follow are actively reading.
func (r *FileRepository) GetSocialPicks(ctx context.Context, userID string, limit int) ([]*domain.File, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := r.db.Pool.Query(ctx, `
		SELECT f.id, f.user_id, f.filename,
		       COALESCE(f.title, f.filename), COALESCE(f.author_name, ''), COALESCE(f.language, ''),
		       f.file_url, f.mime_type, f.file_size,
		       COALESCE(f.category_id::text, ''), f.downloads_count, f.likes_count, f.is_previewable,
		       COALESCE(f.preview_url, ''), COALESCE(f.cover_url, ''), COALESCE(f.description, ''),
		       COALESCE(f.pages_count, 0), COALESCE(f.doc_format, ''),
		       COALESCE(f.pdf_conversion_status, 'none'),
		       f.created_at,
		       u.id, u.username, u.full_name, u.avatar_url, u.is_verified
		FROM files f
		JOIN users u ON u.id = f.user_id
		JOIN (
		    SELECT rs.file_id, COUNT(DISTINCT rs.user_id) AS cnt
		    FROM reading_status rs
		    JOIN follows fl ON fl.following_id = rs.user_id
		    WHERE fl.follower_id = $1
		      AND rs.status IN ('reading', 'done')
		    GROUP BY rs.file_id
		) picks ON picks.file_id = f.id
		ORDER BY picks.cnt DESC, f.downloads_count DESC
		LIMIT $2`,
		userID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("get social picks: %w", err)
	}
	defer rows.Close()
	return scanFiles(rows)
}

// GetRelatedFiles returns files by the same author or same category, excluding fileID.
func (r *FileRepository) GetRelatedFiles(ctx context.Context, fileID string, limit int) ([]*domain.File, error) {
	if limit <= 0 {
		limit = 8
	}
	// Fetch author_name and category_id of the target file
	var authorName, categoryID string
	_ = r.db.Pool.QueryRow(ctx,
		`SELECT COALESCE(author_name, ''), COALESCE(category_id::text, '') FROM files WHERE id = $1`,
		fileID,
	).Scan(&authorName, &categoryID)

	rows, err := r.db.Pool.Query(ctx, `
		SELECT f.id, f.user_id, f.filename,
		       COALESCE(f.title, f.filename), COALESCE(f.author_name, ''), COALESCE(f.language, ''),
		       f.file_url, f.mime_type, f.file_size,
		       COALESCE(f.category_id::text, ''), f.downloads_count, f.likes_count, f.is_previewable,
		       COALESCE(f.preview_url, ''), COALESCE(f.cover_url, ''), COALESCE(f.description, ''),
		       COALESCE(f.pages_count, 0), COALESCE(f.doc_format, ''),
		       COALESCE(f.pdf_conversion_status, 'none'),
		       f.created_at,
		       u.id, u.username, u.full_name, u.avatar_url, u.is_verified
		FROM files f
		JOIN users u ON u.id = f.user_id
		WHERE f.id != $1
		  AND (
		      ($2 != '' AND LOWER(COALESCE(f.author_name, '')) = LOWER($2))
		      OR ($3 != '' AND f.category_id::text = $3)
		  )
		ORDER BY
		  CASE WHEN $2 != '' AND LOWER(COALESCE(f.author_name, '')) = LOWER($2) THEN 0 ELSE 1 END,
		  f.downloads_count DESC,
		  COALESCE(f.views_count, 0) DESC
		LIMIT $4`,
		fileID, authorName, categoryID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("get related files: %w", err)
	}
	defer rows.Close()
	return scanFiles(rows)
}

// SearchSuggestions возвращает подсказки (заголовки и авторы) по префиксу.
func (r *FileRepository) SearchSuggestions(ctx context.Context, q string, limit int) ([]map[string]interface{}, error) {
	if limit <= 0 {
		limit = 8
	}
	pattern := "%" + q + "%"

	// Titles matching the query (ranked by likes+downloads for the most popular matching title)
	rows, err := r.db.Pool.Query(ctx, `
		(SELECT 'title' AS type, title AS value, MAX(likes_count + downloads_count) AS score
		 FROM files
		 WHERE title IS NOT NULL AND title ILIKE $1
		 GROUP BY title
		 ORDER BY score DESC, title
		 LIMIT $2)
		UNION ALL
		(SELECT 'author' AS type, author_name AS value, SUM(likes_count + downloads_count) AS score
		 FROM files
		 WHERE author_name IS NOT NULL AND author_name != '' AND author_name ILIKE $1
		 GROUP BY author_name
		 ORDER BY score DESC, author_name
		 LIMIT $2)
		ORDER BY score DESC
	`, pattern, limit)
	if err != nil {
		return nil, fmt.Errorf("search suggestions: %w", err)
	}
	defer rows.Close()

	var suggestions []map[string]interface{}
	for rows.Next() {
		var typ, value string
		var score int
		if err := rows.Scan(&typ, &value, &score); err != nil {
			return nil, err
		}
		// Flutter expects "text" and "type" keys
		suggestions = append(suggestions, map[string]interface{}{
			"type": typ,
			"text": value,
		})
	}
	return suggestions, rows.Err()
}
