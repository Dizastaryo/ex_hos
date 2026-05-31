package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/seeu/backend/internal/domain"
)

type ReportRepository struct {
	db *DB
}

func NewReportRepository(db *DB) *ReportRepository {
	return &ReportRepository{db: db}
}

// Create inserts a fresh report. Reporter cannot file the same target twice with
// the same reason within a short window — but we keep that policy in the service
// layer to keep this repo a thin SQL wrapper.
func (r *ReportRepository) Create(ctx context.Context, rep *domain.Report) error {
	query := `
		INSERT INTO reports (reporter_id, target_type, target_id, reason, details)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, status, created_at`
	if err := r.db.Pool.QueryRow(ctx, query,
		rep.ReporterID, rep.TargetType, rep.TargetID, rep.Reason, rep.Details,
	).Scan(&rep.ID, &rep.Status, &rep.CreatedAt); err != nil {
		return fmt.Errorf("create report: %w", err)
	}
	return nil
}

// AdminList returns reports filtered by status, newest first.
// Pass an empty string to include all statuses.
//
// The query enriches each row with a "target_preview" map so the moderator
// can see what they're acting on without an extra round-trip:
//   - post:    caption, first media_url + media_type, author_username
//   - comment: text, author_username, post_id, post caption snippet
//   - story:   media_url + media_type, author_username
//   - user:    username, full_name, avatar_url
//
// LEFT JOINs are gated on r.target_type so unrelated rows don't match
// (UUIDs are unique across tables but the type guard is explicit).
func (r *ReportRepository) AdminList(ctx context.Context, status string, limit, offset int) ([]map[string]any, error) {
	if limit <= 0 {
		limit = 50
	}
	q := `
		SELECT r.id, r.target_type, r.target_id, r.reason, r.details, r.status,
		       r.created_at, r.reviewed_at,
		       ru.id, ru.username, ru.avatar_url,
		       -- post target
		       p.caption AS post_caption,
		       (CASE WHEN array_length(p.media_urls, 1) > 0 THEN p.media_urls[1] END) AS post_media_url,
		       (CASE WHEN array_length(p.media_types, 1) > 0 THEN p.media_types[1] END) AS post_media_type,
		       pu.username AS post_author,
		       -- comment target
		       cm.text AS comment_text,
		       cm.post_id AS comment_post_id,
		       cu.username AS comment_author,
		       -- story target
		       s.media_url AS story_media_url,
		       s.media_type AS story_media_type,
		       su.username AS story_author,
		       -- user target
		       tu.username AS user_username,
		       tu.full_name AS user_full_name,
		       tu.avatar_url AS user_avatar
		FROM reports r
		JOIN users ru ON ru.id = r.reporter_id
		LEFT JOIN posts p ON r.target_type = 'post' AND p.id = r.target_id
		LEFT JOIN users pu ON pu.id = p.user_id
		LEFT JOIN comments cm ON r.target_type = 'comment' AND cm.id = r.target_id
		LEFT JOIN users cu ON cu.id = cm.user_id
		LEFT JOIN stories s ON r.target_type = 'story' AND s.id = r.target_id
		LEFT JOIN users su ON su.id = s.user_id
		LEFT JOIN users tu ON r.target_type = 'user' AND tu.id = r.target_id`
	args := []any{}
	if status != "" && status != "all" {
		args = append(args, status)
		q += " WHERE r.status = $1"
	}
	args = append(args, limit, offset)
	q += fmt.Sprintf(" ORDER BY r.created_at DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args))

	rows, err := r.db.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("admin list reports: %w", err)
	}
	defer rows.Close()

	var out []map[string]any
	for rows.Next() {
		var (
			id, ttype, tid, reason, details, st       string
			created                                   time.Time
			reviewed                                  *time.Time
			reporterID, reporterUsername, reporterAvi string

			postCaption, postMediaURL, postMediaType, postAuthor *string
			commentText, commentPostID, commentAuthor            *string
			storyMediaURL, storyMediaType, storyAuthor           *string
			userUsername, userFullName, userAvatar               *string
		)
		if err := rows.Scan(&id, &ttype, &tid, &reason, &details, &st, &created, &reviewed,
			&reporterID, &reporterUsername, &reporterAvi,
			&postCaption, &postMediaURL, &postMediaType, &postAuthor,
			&commentText, &commentPostID, &commentAuthor,
			&storyMediaURL, &storyMediaType, &storyAuthor,
			&userUsername, &userFullName, &userAvatar,
		); err != nil {
			return nil, fmt.Errorf("admin list reports scan: %w", err)
		}

		var preview map[string]any
		switch ttype {
		case "post":
			if postAuthor != nil {
				preview = map[string]any{
					"caption":         deref(postCaption),
					"media_url":       deref(postMediaURL),
					"media_type":      deref(postMediaType),
					"author_username": deref(postAuthor),
				}
			}
		case "comment":
			if commentAuthor != nil {
				preview = map[string]any{
					"text":            deref(commentText),
					"post_id":         deref(commentPostID),
					"author_username": deref(commentAuthor),
				}
			}
		case "story":
			if storyAuthor != nil {
				preview = map[string]any{
					"media_url":       deref(storyMediaURL),
					"media_type":      deref(storyMediaType),
					"author_username": deref(storyAuthor),
				}
			}
		case "user":
			if userUsername != nil {
				preview = map[string]any{
					"username":   deref(userUsername),
					"full_name":  deref(userFullName),
					"avatar_url": deref(userAvatar),
				}
			}
		}

		out = append(out, map[string]any{
			"id":             id,
			"target_type":    ttype,
			"target_id":      tid,
			"target_preview": preview,
			"reason":         reason,
			"details":        details,
			"status":         st,
			"created_at":     created,
			"reviewed_at":    reviewed,
			"reporter": map[string]any{
				"id":         reporterID,
				"username":   reporterUsername,
				"avatar_url": reporterAvi,
			},
		})
	}
	return out, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// UpdateStatus closes a report. status is expected to be "dismissed" or "actioned".
func (r *ReportRepository) UpdateStatus(ctx context.Context, id, status, reviewerID string) error {
	tag, err := r.db.Pool.Exec(ctx,
		`UPDATE reports SET status = $1, reviewed_at = NOW(), reviewed_by = $2 WHERE id = $3`,
		status, reviewerID, id)
	if err != nil {
		return fmt.Errorf("update report: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("report not found")
	}
	return nil
}

// AdminMetrics aggregates dashboard counts in one round-trip.
func (r *ReportRepository) AdminMetrics(ctx context.Context) (map[string]int, error) {
	// One query with sub-selects keeps the round-trip cheap.
	var users, usersToday, posts, postsToday, storiesActive, reportsPending, reportsToday int
	err := r.db.Pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM users)                                                  AS users,
			(SELECT COUNT(*) FROM users WHERE created_at > NOW() - INTERVAL '1 day')      AS users_today,
			(SELECT COUNT(*) FROM posts)                                                  AS posts,
			(SELECT COUNT(*) FROM posts WHERE created_at > NOW() - INTERVAL '1 day')      AS posts_today,
			(SELECT COUNT(*) FROM stories WHERE expires_at > NOW())                       AS stories_active,
			(SELECT COUNT(*) FROM reports WHERE status = 'pending')                       AS reports_pending,
			(SELECT COUNT(*) FROM reports WHERE created_at > NOW() - INTERVAL '1 day')    AS reports_today
	`).Scan(&users, &usersToday, &posts, &postsToday, &storiesActive, &reportsPending, &reportsToday)
	if err != nil {
		return nil, fmt.Errorf("admin metrics: %w", err)
	}
	return map[string]int{
		"users":           users,
		"users_today":     usersToday,
		"posts":           posts,
		"posts_today":     postsToday,
		"stories_active":  storiesActive,
		"reports_pending": reportsPending,
		"reports_today":   reportsToday,
	}, nil
}

// DailyMetric is one bucket of the admin time-series chart.
type DailyMetric struct {
	Day     string `json:"day"` // YYYY-MM-DD
	DAU     int    `json:"dau"`
	Signups int    `json:"signups"`
	Posts   int    `json:"posts"`
}

// AdminTimeSeries returns N days of daily activity (DAU, signups, posts) including
// today, with zero rows for days where nothing happened. days is clamped to [7, 90].
//
// DAU here is "users who did *something* observable" — created post/comment/like
// or viewed a story. We don't have a dedicated activity log, so this is a proxy.
// It's stable enough for a moderation dashboard.
func (r *ReportRepository) AdminTimeSeries(ctx context.Context, days int) ([]DailyMetric, error) {
	if days < 7 {
		days = 7
	}
	if days > 90 {
		days = 90
	}

	// One query: generate_series produces all days in the window, then we LEFT JOIN
	// the three aggregations on day. NULLs become 0 via COALESCE.
	q := fmt.Sprintf(`
		WITH days AS (
			SELECT generate_series(
				(NOW() AT TIME ZONE 'UTC')::date - INTERVAL '%d days',
				(NOW() AT TIME ZONE 'UTC')::date,
				INTERVAL '1 day'
			)::date AS day
		),
		activity AS (
			SELECT DISTINCT user_id, created_at::date AS day FROM posts
			WHERE created_at > NOW() - INTERVAL '%d days'
			UNION
			SELECT DISTINCT user_id, created_at::date FROM comments
			WHERE created_at > NOW() - INTERVAL '%d days'
			UNION
			SELECT DISTINCT user_id, created_at::date FROM likes
			WHERE created_at > NOW() - INTERVAL '%d days'
			UNION
			SELECT DISTINCT user_id, viewed_at::date FROM story_views
			WHERE viewed_at > NOW() - INTERVAL '%d days'
		),
		dau AS (
			SELECT day, COUNT(DISTINCT user_id) AS cnt FROM activity GROUP BY day
		),
		signups AS (
			SELECT created_at::date AS day, COUNT(*) AS cnt FROM users
			WHERE created_at > NOW() - INTERVAL '%d days'
			GROUP BY day
		),
		new_posts AS (
			SELECT created_at::date AS day, COUNT(*) AS cnt FROM posts
			WHERE created_at > NOW() - INTERVAL '%d days'
			GROUP BY day
		)
		SELECT d.day, COALESCE(a.cnt, 0), COALESCE(s.cnt, 0), COALESCE(p.cnt, 0)
		FROM days d
		LEFT JOIN dau a ON a.day = d.day
		LEFT JOIN signups s ON s.day = d.day
		LEFT JOIN new_posts p ON p.day = d.day
		ORDER BY d.day`,
		days-1, days, days, days, days, days, days)

	rows, err := r.db.Pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("admin time series: %w", err)
	}
	defer rows.Close()

	out := make([]DailyMetric, 0, days)
	for rows.Next() {
		var day time.Time
		var dau, signups, posts int
		if err := rows.Scan(&day, &dau, &signups, &posts); err != nil {
			return nil, err
		}
		out = append(out, DailyMetric{
			Day:     day.Format("2006-01-02"),
			DAU:     dau,
			Signups: signups,
			Posts:   posts,
		})
	}
	return out, nil
}

// CountRecentByReporter helps rate-limit reports per user (anti-abuse).
func (r *ReportRepository) CountRecentByReporter(ctx context.Context, reporterID string) (int, error) {
	var n int
	err := r.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM reports
		WHERE reporter_id = $1 AND created_at > NOW() - INTERVAL '1 hour'`,
		reporterID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count recent reports: %w", err)
	}
	return n, nil
}
