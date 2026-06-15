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

// GetReadingStatusBatch returns a map[fileID]status for the given user and file IDs.
func (r *ReadingRepository) GetReadingStatusBatch(ctx context.Context, userID string, fileIDs []string) (map[string]string, error) {
	if len(fileIDs) == 0 {
		return nil, nil
	}
	rows, err := r.db.Pool.Query(ctx,
		`SELECT file_id, status FROM reading_status WHERE user_id = $1 AND file_id = ANY($2)`,
		userID, fileIDs)
	if err != nil {
		return nil, fmt.Errorf("batch reading status: %w", err)
	}
	defer rows.Close()
	result := make(map[string]string)
	for rows.Next() {
		var fid, status string
		if err := rows.Scan(&fid, &status); err != nil {
			return nil, err
		}
		result[fid] = status
	}
	return result, nil
}

// GetRecentlyRead returns files recently read by the user, ordered by last_read_at DESC.
func (r *ReadingRepository) GetRecentlyRead(ctx context.Context, userID string, limit int) ([]*domain.File, error) {
	if limit <= 0 {
		limit = 10
	}
	query := `
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
		JOIN reading_progress rp ON rp.file_id = f.id AND rp.user_id = $1
		ORDER BY rp.last_read_at DESC
		LIMIT $2`
	rows, err := r.db.Pool.Query(ctx, query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("recently read: %w", err)
	}
	defer rows.Close()
	return scanFiles(rows)
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

// GetReadingStats возвращает агрегированную статистику чтения пользователя.
func (r *ReadingRepository) GetReadingStats(ctx context.Context, userID string) (map[string]interface{}, error) {
	stats := map[string]interface{}{
		"books_reading":   0,
		"books_done":      0,
		"books_want":      0,
		"total_bookmarks": 0,
		"last_read_at":    nil,
	}

	// Counts by status (only books category)
	rows, err := r.db.Pool.Query(ctx,
		`SELECT rs.status, COUNT(*)
		 FROM reading_status rs
		 JOIN files f ON f.id = rs.file_id
		 JOIN file_categories fc ON fc.id = f.category_id AND fc.slug = 'books'
		 WHERE rs.user_id = $1
		 GROUP BY rs.status`, userID)
	if err != nil {
		return nil, fmt.Errorf("reading stats statuses: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		switch status {
		case "reading":
			stats["books_reading"] = count
		case "done":
			stats["books_done"] = count
		case "want":
			stats["books_want"] = count
		}
	}

	// Total bookmarks
	var bmCount int
	err = r.db.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM file_bookmarks WHERE user_id = $1`, userID).Scan(&bmCount)
	if err != nil {
		return nil, fmt.Errorf("reading stats bookmarks: %w", err)
	}
	stats["total_bookmarks"] = bmCount

	// Last read timestamp
	var lastReadAt *string
	err = r.db.Pool.QueryRow(ctx,
		`SELECT MAX(last_read_at)::text FROM reading_progress WHERE user_id = $1`, userID).Scan(&lastReadAt)
	if err == nil && lastReadAt != nil {
		stats["last_read_at"] = *lastReadAt
	}

	// Total pages read (only books, from page_reading_progress with threshold)
	var totalPages int
	err = r.db.Pool.QueryRow(ctx,
		`SELECT COUNT(*)
		 FROM page_reading_progress prp
		 JOIN files f ON f.id = prp.file_id
		 JOIN file_categories fc ON fc.id = f.category_id AND fc.slug = 'books'
		 WHERE prp.user_id = $1 AND prp.seconds_spent >= $2`,
		userID, PageReadThreshold).Scan(&totalPages)
	if err == nil {
		stats["total_pages_read"] = totalPages
	}

	// Total library files liked by user
	var totalLikes int
	err = r.db.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM likes WHERE user_id = $1 AND entity_type = 'file'`, userID).Scan(&totalLikes)
	if err == nil {
		stats["total_likes"] = totalLikes
	}

	// Total files rated by user
	var totalRatings int
	err = r.db.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM file_ratings WHERE user_id = $1`, userID).Scan(&totalRatings)
	if err == nil {
		stats["total_ratings"] = totalRatings
	}

	// Total files viewed (history)
	var totalViewed int
	err = r.db.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM file_views WHERE user_id = $1`, userID).Scan(&totalViewed)
	if err == nil {
		stats["total_viewed"] = totalViewed
	}

	// Reading streak (consecutive calendar days — only books)
	streakRows, streakErr := r.db.Pool.Query(ctx,
		`SELECT DISTINCT DATE(rp.last_read_at AT TIME ZONE 'UTC')
		 FROM reading_progress rp
		 JOIN files f ON f.id = rp.file_id
		 JOIN file_categories fc ON fc.id = f.category_id AND fc.slug = 'books'
		 WHERE rp.user_id = $1
		 ORDER BY 1 DESC`, userID)
	if streakErr == nil {
		defer streakRows.Close()
		var dates []string
		for streakRows.Next() {
			var d string
			if e := streakRows.Scan(&d); e == nil {
				dates = append(dates, d[:10]) // YYYY-MM-DD
			}
		}
		stats["reading_streak"] = computeReadingStreak(dates)
	}

	return stats, nil
}

// computeReadingStreak counts how many consecutive days appear in dates.
// dates are YYYY-MM-DD strings sorted descending (most recent first), unique.
func computeReadingStreak(dates []string) int {
	if len(dates) == 0 {
		return 0
	}
	streak := 1
	for i := 1; i < len(dates); i++ {
		diff := readingDateDiffDays(dates[i-1], dates[i])
		if diff == 1 {
			streak++
		} else {
			break
		}
	}
	return streak
}

// readingDateDiffDays returns (a - b) in calendar days. Both strings are YYYY-MM-DD.
func readingDateDiffDays(a, b string) int {
	if len(a) < 10 || len(b) < 10 {
		return 99
	}
	return readingDateToEpochDays(a) - readingDateToEpochDays(b)
}

// readingDateToEpochDays converts a YYYY-MM-DD string to days since 0001-01-01.
// Uses the civil / proleptic Gregorian calendar approach.
func readingDateToEpochDays(s string) int {
	y := readingParseInt(s[0:4])
	m := readingParseInt(s[5:7])
	d := readingParseInt(s[8:10])
	// Algorithm from https://stackoverflow.com/a/15667500
	if m < 3 {
		y--
		m += 12
	}
	a := y / 100
	b := 2 - a + a/4
	return int(365.25*float64(y+4716)) + int(30.6001*float64(m+1)) + d + b - 1524
}

func readingParseInt(s string) int {
	v := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			v = v*10 + int(c-'0')
		}
	}
	return v
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
		       COALESCE(f.preview_url, ''), COALESCE(f.cover_url, ''), COALESCE(f.description, ''),
		       COALESCE(f.pages_count, 0), COALESCE(f.doc_format, ''),
		       COALESCE(f.pdf_conversion_status, 'none'),
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
		nextCursor = files[limit].ID
		files = files[:limit]
	}
	return files, nextCursor, nil
}

// GetReadingLeaderboard returns top readers sorted by the given metric.
// metric: "books" | "pages" | "streak"
func (r *ReadingRepository) GetReadingLeaderboard(ctx context.Context, metric string, limit int) ([]map[string]interface{}, error) {
	if limit <= 0 {
		limit = 20
	}

	if metric == "streak" {
		return r.getStreakLeaderboard(ctx, limit)
	}

	var query string
	switch metric {
	case "pages":
		// Count actually read pages (seconds_spent >= threshold) — only books
		query = fmt.Sprintf(`
			SELECT u.id, u.username, u.full_name, COALESCE(u.avatar_url, ''),
			       COALESCE((
			           SELECT COUNT(*) FROM reading_status rs2
			           JOIN files f2 ON f2.id = rs2.file_id
			           JOIN file_categories fc2 ON fc2.id = f2.category_id AND fc2.slug = 'books'
			           WHERE rs2.user_id = u.id AND rs2.status = 'done'
			       ), 0) AS books_done,
			       COUNT(prp.page_number) AS total_pages
			FROM page_reading_progress prp
			JOIN files f ON f.id = prp.file_id
			JOIN file_categories fc ON fc.id = f.category_id AND fc.slug = 'books'
			JOIN users u ON u.id = prp.user_id
			WHERE prp.seconds_spent >= %d
			GROUP BY u.id, u.username, u.full_name, u.avatar_url
			HAVING COUNT(prp.page_number) > 0
			ORDER BY total_pages DESC, books_done DESC
			LIMIT $1`, PageReadThreshold)
	default: // "books"
		query = `
			SELECT u.id, u.username, u.full_name, COALESCE(u.avatar_url, ''),
			       COUNT(rs.file_id) AS books_done,
			       COALESCE(SUM(f.pages_count), 0) AS total_pages
			FROM reading_status rs
			JOIN files f ON f.id = rs.file_id
			JOIN file_categories fc ON fc.id = f.category_id AND fc.slug = 'books'
			JOIN users u ON u.id = rs.user_id
			WHERE rs.status = 'done'
			GROUP BY u.id, u.username, u.full_name, u.avatar_url
			ORDER BY books_done DESC, total_pages DESC
			LIMIT $1`
	}

	rows, err := r.db.Pool.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("reading leaderboard: %w", err)
	}
	defer rows.Close()

	var result []map[string]interface{}
	rank := 1
	for rows.Next() {
		var userID, username, fullName, avatarURL string
		var booksDone, totalPages int
		if err := rows.Scan(&userID, &username, &fullName, &avatarURL, &booksDone, &totalPages); err != nil {
			return nil, err
		}
		result = append(result, map[string]interface{}{
			"rank":        rank,
			"user_id":     userID,
			"username":    username,
			"full_name":   fullName,
			"avatar_url":  avatarURL,
			"books_done":  booksDone,
			"total_pages": totalPages,
		})
		rank++
	}
	return result, rows.Err()
}

// getStreakLeaderboard returns users ranked by their current reading streak.
// Computes streaks in SQL using window functions — returns only users with streak > 0.
func (r *ReadingRepository) getStreakLeaderboard(ctx context.Context, limit int) ([]map[string]interface{}, error) {
	// Get distinct reading days per user, ordered DESC within each user.
	// Then compute streak by checking consecutive day gaps.
	rows, err := r.db.Pool.Query(ctx, `
		WITH user_days AS (
			SELECT rp.user_id, DATE(rp.last_read_at AT TIME ZONE 'UTC') AS day
			FROM reading_progress rp
			JOIN files f ON f.id = rp.file_id
			JOIN file_categories fc ON fc.id = f.category_id AND fc.slug = 'books'
			GROUP BY rp.user_id, day
		),
		ordered AS (
			SELECT user_id, day,
			       ROW_NUMBER() OVER (PARTITION BY user_id ORDER BY day DESC) AS rn,
			       day - (ROW_NUMBER() OVER (PARTITION BY user_id ORDER BY day DESC))::int AS grp
			FROM user_days
		),
		streaks AS (
			SELECT user_id,
			       MAX(CASE WHEN rn = 1 THEN day END) AS last_day,
			       MAX(CASE WHEN grp = (
			           SELECT grp FROM ordered o2
			           WHERE o2.user_id = ordered.user_id
			           ORDER BY o2.rn LIMIT 1
			       ) THEN 1 ELSE 0 END) AS in_current,
			       COUNT(*) FILTER (WHERE grp = (
			           SELECT grp FROM ordered o2
			           WHERE o2.user_id = ordered.user_id
			           ORDER BY o2.rn LIMIT 1
			       )) AS streak_len
			FROM ordered
			GROUP BY user_id
		),
		current_streaks AS (
			SELECT s.user_id, s.streak_len
			FROM streaks s
			WHERE s.last_day >= (NOW() AT TIME ZONE 'UTC')::date - 1
			  AND s.streak_len > 0
		)
		SELECT u.id, u.username, u.full_name, COALESCE(u.avatar_url, ''),
		       cs.streak_len,
		       COALESCE((
		           SELECT COUNT(*) FROM reading_status rs
		           JOIN files f2 ON f2.id = rs.file_id
		           JOIN file_categories fc2 ON fc2.id = f2.category_id AND fc2.slug = 'books'
		           WHERE rs.user_id = u.id AND rs.status = 'done'
		       ), 0)
		FROM current_streaks cs
		JOIN users u ON u.id = cs.user_id
		ORDER BY cs.streak_len DESC, u.username
		LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("streak leaderboard: %w", err)
	}
	defer rows.Close()

	var result []map[string]interface{}
	rank := 1
	for rows.Next() {
		var userID, username, fullName, avatarURL string
		var streakLen, booksDone int
		if err := rows.Scan(&userID, &username, &fullName, &avatarURL, &streakLen, &booksDone); err != nil {
			return nil, err
		}
		result = append(result, map[string]interface{}{
			"rank":        rank,
			"user_id":     userID,
			"username":    username,
			"full_name":   fullName,
			"avatar_url":  avatarURL,
			"books_done":  booksDone,
			"total_pages": 0,
			"streak_days": streakLen,
		})
		rank++
	}
	return result, rows.Err()
}

// GetReadingGoal returns the user's reading goal for the given year (nil if not set).
func (r *ReadingRepository) GetReadingGoal(ctx context.Context, userID string, year int) (*domain.ReadingGoal, error) {
	g := &domain.ReadingGoal{}
	err := r.db.Pool.QueryRow(ctx, `
		SELECT user_id, year, goal_books, updated_at
		FROM reading_goals
		WHERE user_id = $1 AND year = $2`,
		userID, year,
	).Scan(&g.UserID, &g.Year, &g.GoalBooks, &g.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get reading goal: %w", err)
	}
	// Count books finished this year
	err = r.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM reading_status
		WHERE user_id = $1 AND status = 'done'
		  AND EXTRACT(YEAR FROM updated_at) = $2`,
		userID, year,
	).Scan(&g.DoneBooks)
	if err != nil {
		g.DoneBooks = 0
	}
	return g, nil
}

// UpsertReadingGoal sets or updates the reading goal for the user+year.
func (r *ReadingRepository) UpsertReadingGoal(ctx context.Context, userID string, year, goalBooks int) (*domain.ReadingGoal, error) {
	g := &domain.ReadingGoal{}
	err := r.db.Pool.QueryRow(ctx, `
		INSERT INTO reading_goals (user_id, year, goal_books)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, year) DO UPDATE SET goal_books = $3, updated_at = NOW()
		RETURNING user_id, year, goal_books, updated_at`,
		userID, year, goalBooks,
	).Scan(&g.UserID, &g.Year, &g.GoalBooks, &g.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("upsert reading goal: %w", err)
	}
	// Count books finished this year
	_ = r.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM reading_status
		WHERE user_id = $1 AND status = 'done'
		  AND EXTRACT(YEAR FROM updated_at) = $2`,
		userID, year,
	).Scan(&g.DoneBooks)
	return g, nil
}

// DeleteReadingGoal removes the user's reading goal for the given year.
func (r *ReadingRepository) DeleteReadingGoal(ctx context.Context, userID string, year int) error {
	_, err := r.db.Pool.Exec(ctx,
		`DELETE FROM reading_goals WHERE user_id = $1 AND year = $2`, userID, year)
	return err
}

// GetReadingActivity returns daily reading activity for the past N days.
// Returns a slice of {date: "YYYY-MM-DD", sessions: N} objects.
func (r *ReadingRepository) GetReadingActivity(ctx context.Context, userID string, days int) ([]map[string]interface{}, error) {
	if days <= 0 {
		days = 7
	}
	if days > 365 {
		days = 365
	}
	rows, err := r.db.Pool.Query(ctx, `
		SELECT gs.day::text, COALESCE(cnt, 0) AS sessions
		FROM generate_series(
			(NOW() AT TIME ZONE 'UTC')::date - ($2 - 1) * INTERVAL '1 day',
			(NOW() AT TIME ZONE 'UTC')::date,
			'1 day'::interval
		) AS gs(day)
		LEFT JOIN (
			SELECT DATE(rp.last_read_at AT TIME ZONE 'UTC') AS day, COUNT(DISTINCT rp.file_id) AS cnt
			FROM reading_progress rp
			JOIN files f ON f.id = rp.file_id
			JOIN file_categories fc ON fc.id = f.category_id AND fc.slug = 'books'
			WHERE rp.user_id = $1
			  AND rp.last_read_at >= NOW() - $2 * INTERVAL '1 day'
			GROUP BY 1
		) a ON a.day = gs.day
		ORDER BY gs.day ASC`,
		userID, days)
	if err != nil {
		return nil, fmt.Errorf("reading activity: %w", err)
	}
	defer rows.Close()

	var result []map[string]interface{}
	for rows.Next() {
		var day string
		var sessions int
		if err := rows.Scan(&day, &sessions); err != nil {
			return nil, err
		}
		result = append(result, map[string]interface{}{
			"date":     day[:10],
			"sessions": sessions,
		})
	}
	if result == nil {
		result = []map[string]interface{}{}
	}
	return result, rows.Err()
}

// ─── File Notes ───────────────────────────────────────────────────────────────

// GetFileNote returns the user's private note for a file (empty string if none).
func (r *ReadingRepository) GetFileNote(ctx context.Context, userID, fileID string) (string, error) {
	var content string
	err := r.db.Pool.QueryRow(ctx,
		`SELECT content FROM file_notes WHERE user_id = $1 AND file_id = $2`,
		userID, fileID,
	).Scan(&content)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("get file note: %w", err)
	}
	return content, nil
}

// UpsertFileNote creates or updates a personal note.
func (r *ReadingRepository) UpsertFileNote(ctx context.Context, userID, fileID, content string) error {
	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO file_notes (user_id, file_id, content)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, file_id)
		DO UPDATE SET content = EXCLUDED.content, updated_at = NOW()`,
		userID, fileID, content,
	)
	return err
}

// DeleteFileNote removes the user's note for a file.
func (r *ReadingRepository) DeleteFileNote(ctx context.Context, userID, fileID string) error {
	_, err := r.db.Pool.Exec(ctx,
		`DELETE FROM file_notes WHERE user_id = $1 AND file_id = $2`,
		userID, fileID,
	)
	return err
}

// ─── Page Reading Progress (honest reading tracker) ──────────────────────────

const PageReadThreshold = 40 // seconds to count a page as "read"

// booksCategorySlug is the slug used to identify "book" files in file_categories.
// Only books count toward reading stats, leaderboards, and streaks.
const booksCategorySlug = "books"

// booksJoin is a SQL fragment that filters reading_status/reading_progress rows
// to only include files in the "books" category.
const booksFileJoin = `JOIN files _bf ON _bf.id = %s JOIN file_categories _bc ON _bc.id = _bf.category_id AND _bc.slug = 'books'`

// GetPageProgress returns all page reading times for a user+file.
// Result: map[pageNumber]secondsSpent
func (r *ReadingRepository) GetPageProgress(ctx context.Context, userID, fileID string) (map[int]int, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT page_number, seconds_spent
		FROM page_reading_progress
		WHERE user_id = $1 AND file_id = $2`,
		userID, fileID)
	if err != nil {
		return nil, fmt.Errorf("get page progress: %w", err)
	}
	defer rows.Close()

	result := make(map[int]int)
	for rows.Next() {
		var page, seconds int
		if err := rows.Scan(&page, &seconds); err != nil {
			return nil, err
		}
		result[page] = seconds
	}
	return result, rows.Err()
}

// BatchUpsertPageProgress upserts page reading times.
// Uses GREATEST to never decrease seconds_spent (prevents cheating / sync conflicts).
func (r *ReadingRepository) BatchUpsertPageProgress(ctx context.Context, userID, fileID string, pages map[int]int) error {
	if len(pages) == 0 {
		return nil
	}

	batch := &pgx.Batch{}
	for page, seconds := range pages {
		batch.Queue(`
			INSERT INTO page_reading_progress (user_id, file_id, page_number, seconds_spent, updated_at)
			VALUES ($1, $2, $3, $4, NOW())
			ON CONFLICT (user_id, file_id, page_number)
			DO UPDATE SET
				seconds_spent = GREATEST(page_reading_progress.seconds_spent, $4),
				updated_at = NOW()`,
			userID, fileID, page, seconds)
	}

	br := r.db.Pool.SendBatch(ctx, batch)
	defer br.Close()
	for range pages {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("batch upsert page progress: %w", err)
		}
	}
	return nil
}

// CountReadPages returns number of pages with seconds_spent >= threshold.
func (r *ReadingRepository) CountReadPages(ctx context.Context, userID, fileID string) (int, error) {
	var count int
	err := r.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM page_reading_progress
		WHERE user_id = $1 AND file_id = $2 AND seconds_spent >= $3`,
		userID, fileID, PageReadThreshold,
	).Scan(&count)
	return count, err
}

// CountAllReadPages returns total read pages across all files for a user.
func (r *ReadingRepository) CountAllReadPages(ctx context.Context, userID string) (int, error) {
	var count int
	err := r.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM page_reading_progress
		WHERE user_id = $1 AND seconds_spent >= $2`,
		userID, PageReadThreshold,
	).Scan(&count)
	return count, err
}
