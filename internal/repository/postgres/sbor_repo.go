package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/seeu/backend/internal/domain"
)

type SborRepository struct {
	db *DB
}

func NewSborRepository(db *DB) *SborRepository {
	return &SborRepository{db: db}
}

// ─── Create ──────────────────────────────────────────────────────

func (r *SborRepository) Create(ctx context.Context, s *domain.Sbor) error {
	if s.City == "" {
		s.City = "Алматы"
	}
	query := `
		INSERT INTO sbory (host_id, type, category, title, place, city, cover_url, price, description,
		                   scheduled_at, flexible_time, max_slots)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		RETURNING id, created_at, updated_at`

	err := r.db.Pool.QueryRow(ctx, query,
		s.HostID, s.Type, s.Category, s.Title, s.Place, s.City, s.CoverUrl, s.Price, s.Description,
		s.ScheduledAt, s.FlexibleTime, s.MaxSlots,
	).Scan(&s.ID, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create sbor: %w", err)
	}

	// Host автоматически становится organizer
	_, err = r.db.Pool.Exec(ctx,
		`INSERT INTO sbor_members (sbor_id, user_id, role) VALUES ($1,$2,'organizer')`,
		s.ID, s.HostID,
	)
	return err
}

// SetChatID сохраняет chat_id в sbory после создания group-чата.
func (r *SborRepository) SetChatID(ctx context.Context, sborID, chatID string) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE sbory SET chat_id = $2, updated_at = now() WHERE id = $1`,
		sborID, chatID,
	)
	return err
}

// ─── GetByID ─────────────────────────────────────────────────────

func (r *SborRepository) GetByID(ctx context.Context, id, viewerID string) (*domain.Sbor, error) {
	query := `
		SELECT s.id, s.host_id, s.type, s.category, s.title, s.place, s.city,
		       s.cover_url, s.price, s.description, s.scheduled_at, s.flexible_time, s.max_slots,
		       s.is_live, s.is_cancelled, s.created_at, s.updated_at,
		       s.chat_id,
		       u.full_name,
		       COUNT(DISTINCT sm.user_id)::int AS joined,
		       COALESCE(
		         (SELECT sm2.role FROM sbor_members sm2
		          WHERE sm2.sbor_id = s.id AND sm2.user_id = $2), ''
		       ) AS my_role,
		       EXISTS(
		         SELECT 1 FROM sbor_members sm3
		         WHERE sm3.sbor_id = s.id AND sm3.user_id = $2
		       ) AS is_joined,
		       EXISTS(
		         SELECT 1 FROM sbor_bookmarks sb
		         WHERE sb.sbor_id = s.id AND sb.user_id = $2
		       ) AS is_bookmarked,
		       COALESCE(
		         (SELECT rq.status FROM sbor_requests rq
		          WHERE rq.sbor_id = s.id AND rq.user_id = $2), ''
		       ) AS my_request_status,
		       (SELECT COUNT(*)::int FROM sbor_requests rq2
		        WHERE rq2.sbor_id = s.id AND rq2.status = 'pending') AS pending_requests_count
		FROM sbory s
		JOIN users u ON u.id = s.host_id
		LEFT JOIN sbor_members sm ON sm.sbor_id = s.id
		WHERE s.id = $1 AND NOT s.is_cancelled
		GROUP BY s.id, u.full_name`

	s := &domain.Sbor{}
	err := r.db.Pool.QueryRow(ctx, query, id, viewerID).Scan(
		&s.ID, &s.HostID, &s.Type, &s.Category, &s.Title, &s.Place, &s.City,
		&s.CoverUrl, &s.Price, &s.Description, &s.ScheduledAt, &s.FlexibleTime, &s.MaxSlots,
		&s.IsLive, &s.IsCancelled, &s.CreatedAt, &s.UpdatedAt,
		&s.ChatID,
		&s.HostName, &s.Joined, &s.MyRole, &s.IsJoined, &s.IsBookmarked,
		&s.MyRequestStatus, &s.PendingRequestsCount,
	)
	if err == pgx.ErrNoRows {
		return nil, domain.ErrSborNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get sbor: %w", err)
	}

	s.MemberNames, s.MemberUsernames, s.MemberIDs, s.MemberAvatarURLs, _ = r.getMemberUsers(ctx, id)
	s.When, s.WhenSub = formatWhen(s.ScheduledAt, s.FlexibleTime)
	return s, nil
}

// ─── List (feed) ──────────────────────────────────────────────────

func (r *SborRepository) List(ctx context.Context, viewerID, typeFilter, catFilter, cityFilter, q string, dateFrom, dateTo *time.Time, limit, offset int) ([]*domain.Sbor, error) {
	args := []any{viewerID, limit, offset}
	where := "NOT s.is_cancelled AND (s.scheduled_at IS NULL OR s.scheduled_at > NOW() OR s.is_live = TRUE)"
	idx := 4

	if typeFilter != "" {
		where += fmt.Sprintf(" AND s.type = $%d", idx)
		args = append(args, typeFilter)
		idx++
	}
	if catFilter != "" {
		where += fmt.Sprintf(" AND s.category = $%d", idx)
		args = append(args, catFilter)
		idx++
	}
	if cityFilter != "" {
		where += fmt.Sprintf(" AND s.city = $%d", idx)
		args = append(args, cityFilter)
		idx++
	}
	if dateFrom != nil {
		where += fmt.Sprintf(" AND s.scheduled_at >= $%d", idx)
		args = append(args, dateFrom.UTC())
		idx++
	}
	if dateTo != nil {
		where += fmt.Sprintf(" AND s.scheduled_at <= $%d", idx)
		args = append(args, dateTo.UTC())
		idx++
	}
	if q != "" {
		where += fmt.Sprintf(" AND (s.title ILIKE $%d OR s.place ILIKE $%d)", idx, idx)
		args = append(args, "%"+q+"%")
		idx++
	}
	_ = idx

	query := fmt.Sprintf(`
		SELECT s.id, s.host_id, s.type, s.category, s.title, s.place, s.city,
		       s.cover_url, s.price, s.description, s.scheduled_at, s.flexible_time, s.max_slots,
		       s.is_live, s.created_at, s.chat_id,
		       u.full_name,
		       COUNT(DISTINCT sm.user_id)::int AS joined,
		       COALESCE(
		         (SELECT sm2.role FROM sbor_members sm2
		          WHERE sm2.sbor_id = s.id AND sm2.user_id = $1), ''
		       ) AS my_role,
		       EXISTS(
		         SELECT 1 FROM sbor_members sm3
		         WHERE sm3.sbor_id = s.id AND sm3.user_id = $1
		       ) AS is_joined,
		       EXISTS(
		         SELECT 1 FROM sbor_bookmarks sb
		         WHERE sb.sbor_id = s.id AND sb.user_id = $1
		       ) AS is_bookmarked,
		       COALESCE(
		         (SELECT rq.status FROM sbor_requests rq
		          WHERE rq.sbor_id = s.id AND rq.user_id = $1), ''
		       ) AS my_request_status,
		       (SELECT COUNT(*)::int FROM sbor_requests rq2
		        WHERE rq2.sbor_id = s.id AND rq2.status = 'pending') AS pending_requests_count
		FROM sbory s
		JOIN users u ON u.id = s.host_id
		LEFT JOIN sbor_members sm ON sm.sbor_id = s.id
		WHERE %s
		GROUP BY s.id, u.full_name
		ORDER BY s.is_live DESC, s.created_at DESC
		LIMIT $2 OFFSET $3`, where)

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list sbory: %w", err)
	}
	defer rows.Close()

	var result []*domain.Sbor
	for rows.Next() {
		s := &domain.Sbor{}
		if err := rows.Scan(
			&s.ID, &s.HostID, &s.Type, &s.Category, &s.Title, &s.Place, &s.City,
			&s.CoverUrl, &s.Price, &s.Description, &s.ScheduledAt, &s.FlexibleTime, &s.MaxSlots,
			&s.IsLive, &s.CreatedAt, &s.ChatID,
			&s.HostName, &s.Joined, &s.MyRole, &s.IsJoined, &s.IsBookmarked,
			&s.MyRequestStatus, &s.PendingRequestsCount,
		); err != nil {
			return nil, err
		}
		s.When, s.WhenSub = formatWhen(s.ScheduledAt, s.FlexibleTime)
		result = append(result, s)
	}

	// Batch-load member names (one query for all sbors, not N+1)
	if len(result) > 0 {
		ids := make([]string, len(result))
		idx := make(map[string]int, len(result))
		for i, s := range result {
			ids[i] = s.ID
			idx[s.ID] = i
		}
		memberRows, err := r.db.Pool.Query(ctx, `
			SELECT sm.sbor_id, u.full_name, COALESCE(u.username, ''), u.id, COALESCE(u.avatar_url, '')
			FROM sbor_members sm
			JOIN users u ON u.id = sm.user_id
			WHERE sm.sbor_id = ANY($1)
			ORDER BY sm.sbor_id, sm.joined_at
		`, ids)
		if err == nil {
			defer memberRows.Close()
			counters := make(map[string]int, len(result))
			for memberRows.Next() {
				var sborID, name, username, userID, avatarURL string
				if err := memberRows.Scan(&sborID, &name, &username, &userID, &avatarURL); err != nil {
					continue
				}
				if counters[sborID] >= 8 {
					continue
				}
				counters[sborID]++
				i := idx[sborID]
				result[i].MemberNames = append(result[i].MemberNames, name)
				result[i].MemberUsernames = append(result[i].MemberUsernames, username)
				result[i].MemberIDs = append(result[i].MemberIDs, userID)
				result[i].MemberAvatarURLs = append(result[i].MemberAvatarURLs, avatarURL)
			}
		}
	}

	return result, nil
}

// ─── My sbory ────────────────────────────────────────────────────

func (r *SborRepository) ListMine(ctx context.Context, userID string, past bool, limit, offset int) ([]*domain.Sbor, error) {
	timeFilter := "(s.scheduled_at IS NULL OR s.scheduled_at > NOW() OR s.is_live = TRUE)"
	orderBy := "s.scheduled_at ASC NULLS LAST, s.created_at DESC"
	if past {
		timeFilter = "s.scheduled_at IS NOT NULL AND s.scheduled_at < NOW()"
		orderBy = "s.scheduled_at DESC"
	}
	query := fmt.Sprintf(`
		SELECT s.id, s.host_id, s.type, s.category, s.title, s.place,
		       s.cover_url, s.price, s.description, s.scheduled_at, s.flexible_time, s.max_slots,
		       s.is_live, s.created_at, s.chat_id,
		       u.full_name,
		       COUNT(DISTINCT sm.user_id)::int AS joined,
		       COALESCE(
		         (SELECT sm2.role FROM sbor_members sm2
		          WHERE sm2.sbor_id = s.id AND sm2.user_id = $1), ''
		       ) AS my_role,
		       TRUE AS is_joined,
		       EXISTS(
		         SELECT 1 FROM sbor_bookmarks sb
		         WHERE sb.sbor_id = s.id AND sb.user_id = $1
		       ) AS is_bookmarked,
		       '' AS my_request_status,
		       (SELECT COUNT(*)::int FROM sbor_requests rq2
		        WHERE rq2.sbor_id = s.id AND rq2.status = 'pending') AS pending_requests_count
		FROM sbory s
		JOIN users u ON u.id = s.host_id
		JOIN sbor_members my_m ON my_m.sbor_id = s.id AND my_m.user_id = $1
		LEFT JOIN sbor_members sm ON sm.sbor_id = s.id
		WHERE NOT s.is_cancelled AND %s
		GROUP BY s.id, u.full_name
		ORDER BY %s
		LIMIT $2 OFFSET $3`, timeFilter, orderBy)

	rows, err := r.db.Pool.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list my sbory: %w", err)
	}
	defer rows.Close()

	var result []*domain.Sbor
	for rows.Next() {
		s := &domain.Sbor{}
		if err := rows.Scan(
			&s.ID, &s.HostID, &s.Type, &s.Category, &s.Title, &s.Place,
			&s.CoverUrl, &s.Price, &s.Description, &s.ScheduledAt, &s.FlexibleTime, &s.MaxSlots,
			&s.IsLive, &s.CreatedAt, &s.ChatID,
			&s.HostName, &s.Joined, &s.MyRole, &s.IsJoined, &s.IsBookmarked,
			&s.MyRequestStatus, &s.PendingRequestsCount,
		); err != nil {
			return nil, err
		}
		s.When, s.WhenSub = formatWhen(s.ScheduledAt, s.FlexibleTime)
		result = append(result, s)
	}

	if len(result) > 0 {
		ids := make([]string, len(result))
		idx := make(map[string]int, len(result))
		for i, s := range result {
			ids[i] = s.ID
			idx[s.ID] = i
		}
		memberRows, err := r.db.Pool.Query(ctx, `
			SELECT sm.sbor_id, u.full_name, COALESCE(u.username, ''), u.id, COALESCE(u.avatar_url, '')
			FROM sbor_members sm
			JOIN users u ON u.id = sm.user_id
			WHERE sm.sbor_id = ANY($1)
			ORDER BY sm.sbor_id, sm.joined_at
		`, ids)
		if err == nil {
			defer memberRows.Close()
			counters := make(map[string]int, len(result))
			for memberRows.Next() {
				var sborID, name, username, userID, avatarURL string
				if err := memberRows.Scan(&sborID, &name, &username, &userID, &avatarURL); err != nil {
					continue
				}
				if counters[sborID] >= 8 {
					continue
				}
				counters[sborID]++
				i := idx[sborID]
				result[i].MemberNames = append(result[i].MemberNames, name)
				result[i].MemberUsernames = append(result[i].MemberUsernames, username)
				result[i].MemberIDs = append(result[i].MemberIDs, userID)
				result[i].MemberAvatarURLs = append(result[i].MemberAvatarURLs, avatarURL)
			}
		}
	}
	return result, nil
}

// ─── Join / Leave ────────────────────────────────────────────────

func (r *SborRepository) Join(ctx context.Context, sborID, userID string) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Lock the sbor row and check max_slots atomically to prevent race conditions.
	var joined int
	var maxSlots *int
	err = tx.QueryRow(ctx,
		`SELECT COUNT(sm.user_id)::int, s.max_slots
		 FROM sbory s
		 LEFT JOIN sbor_members sm ON sm.sbor_id = s.id
		 WHERE s.id = $1
		 GROUP BY s.id, s.max_slots
		 FOR UPDATE OF s`,
		sborID,
	).Scan(&joined, &maxSlots)
	if err == pgx.ErrNoRows {
		return domain.ErrSborNotFound
	}
	if err != nil {
		return fmt.Errorf("join check: %w", err)
	}
	if maxSlots != nil && joined >= *maxSlots {
		return domain.ErrSborFull
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO sbor_members (sbor_id, user_id, role) VALUES ($1,$2,'participant')
		 ON CONFLICT DO NOTHING`,
		sborID, userID,
	)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *SborRepository) Leave(ctx context.Context, sborID, userID string) error {
	tag, err := r.db.Pool.Exec(ctx,
		`DELETE FROM sbor_members WHERE sbor_id=$1 AND user_id=$2 AND role='participant'`,
		sborID, userID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotJoined
	}
	return nil
}

// ─── Update / Delete ─────────────────────────────────────────────

func (r *SborRepository) Update(ctx context.Context, id, hostID string, req *domain.UpdateSborRequest) error {
	query := `
		UPDATE sbory SET
		  title        = COALESCE($3, title),
		  place        = COALESCE($4, place),
		  cover_url    = COALESCE($5, cover_url),
		  description  = COALESCE($6, description),
		  price        = COALESCE($7, price),
		  category     = COALESCE($8, category),
		  scheduled_at = CASE
		                   WHEN $9 IS NOT DISTINCT FROM TRUE THEN NULL
		                   WHEN $10::bool THEN $11
		                   ELSE scheduled_at
		                 END,
		  flexible_time= COALESCE($9, flexible_time),
		  max_slots    = CASE WHEN $12::bool THEN $13 ELSE max_slots END,
		  updated_at   = now()
		WHERE id=$1 AND host_id=$2`

	_, err := r.db.Pool.Exec(ctx, query,
		id, hostID,
		req.Title, req.Place, req.CoverUrl, req.Description,
		req.Price, req.Category,
		req.FlexibleTime,
		req.ScheduledAt != nil, req.ScheduledAt,
		req.MaxSlots != nil, req.MaxSlots,
	)
	return err
}

// GetSborByChatID возвращает sborID и hostID по chat_id.
// Если чат не является чатом сбора — возвращает пустые строки без ошибки.
func (r *SborRepository) GetSborByChatID(ctx context.Context, chatID string) (sborID, hostID string, err error) {
	err = r.db.Pool.QueryRow(ctx,
		`SELECT id, host_id FROM sbory WHERE chat_id = $1 AND NOT is_cancelled`,
		chatID,
	).Scan(&sborID, &hostID)
	if err == pgx.ErrNoRows {
		return "", "", nil
	}
	return sborID, hostID, err
}

// UpdateByChatID синхронизирует title и cover_url сбора когда меняется
// связанный групповой чат (bidirectional sync).
func (r *SborRepository) UpdateByChatID(ctx context.Context, chatID, title, coverURL string) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE sbory
		    SET title     = CASE WHEN $2 = '' THEN title     ELSE $2 END,
		        cover_url = CASE WHEN $3 = '' THEN cover_url ELSE $3 END,
		        updated_at = now()
		  WHERE chat_id = $1 AND NOT is_cancelled`,
		chatID, title, coverURL,
	)
	return err
}

func (r *SborRepository) Cancel(ctx context.Context, id, hostID string) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Mark cancelled and retrieve linked chat in one statement.
	var chatID *string
	err = tx.QueryRow(ctx,
		`UPDATE sbory SET is_cancelled=true, updated_at=now()
		  WHERE id=$1 AND host_id=$2
		  RETURNING chat_id`,
		id, hostID,
	).Scan(&chatID)
	if err == pgx.ErrNoRows {
		return domain.ErrSborNotFound
	}
	if err != nil {
		return fmt.Errorf("cancel sbor: %w", err)
	}

	// Delete the linked group chat atomically so no orphan chat can remain.
	if chatID != nil {
		_, err = tx.Exec(ctx,
			`DELETE FROM conversations WHERE id=$1`,
			*chatID,
		)
		if err != nil {
			return fmt.Errorf("delete sbor chat: %w", err)
		}
	}

	return tx.Commit(ctx)
}

// ─── Count helpers ────────────────────────────────────────────────

func (r *SborRepository) CountMembers(ctx context.Context, sborID string) (int, error) {
	var n int
	err := r.db.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM sbor_members WHERE sbor_id=$1`, sborID,
	).Scan(&n)
	return n, err
}

func (r *SborRepository) getMemberUsers(ctx context.Context, sborID string) (names, usernames, ids, avatarURLs []string, err error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT u.full_name, COALESCE(u.username, ''), u.id, COALESCE(u.avatar_url, '')
		FROM sbor_members sm
		JOIN users u ON u.id = sm.user_id
		WHERE sm.sbor_id = $1
		ORDER BY sm.joined_at
		LIMIT 8`, sborID)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var name, username, id, avatarURL string
		if err := rows.Scan(&name, &username, &id, &avatarURL); err != nil {
			return nil, nil, nil, nil, err
		}
		names = append(names, name)
		usernames = append(usernames, username)
		ids = append(ids, id)
		avatarURLs = append(avatarURLs, avatarURL)
	}
	return names, usernames, ids, avatarURLs, nil
}

// SborMemberDTO is a flat view of a sbor participant returned by ListMembers.
type SborMemberDTO struct {
	UserID    string `json:"user_id"`
	FullName  string `json:"full_name"`
	Username  string `json:"username"`
	AvatarURL string `json:"avatar_url"`
	Role      string `json:"role"`
}

// ListMembers returns all participants of a sbor (no LIMIT).
func (r *SborRepository) ListMembers(ctx context.Context, sborID string) ([]*SborMemberDTO, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT u.id, u.full_name, COALESCE(u.username, ''), COALESCE(u.avatar_url, ''), sm.role
		FROM sbor_members sm
		JOIN users u ON u.id = sm.user_id
		WHERE sm.sbor_id = $1
		ORDER BY CASE sm.role WHEN 'organizer' THEN 0 ELSE 1 END, sm.joined_at`,
		sborID,
	)
	if err != nil {
		return nil, fmt.Errorf("list sbor members: %w", err)
	}
	defer rows.Close()
	var result []*SborMemberDTO
	for rows.Next() {
		m := &SborMemberDTO{}
		if err := rows.Scan(&m.UserID, &m.FullName, &m.Username, &m.AvatarURL, &m.Role); err != nil {
			return nil, err
		}
		result = append(result, m)
	}
	return result, rows.Err()
}

// ─── formatWhen ───────────────────────────────────────────────────

func formatWhen(t *time.Time, flexible bool) (when, whenSub string) {
	if flexible || t == nil {
		return "Гибко", "договоримся"
	}
	now := time.Now().UTC()
	tUTC := t.UTC()
	diff := tUTC.Sub(now)
	days := int(diff.Hours() / 24)
	timeStr := fmt.Sprintf("%02d:%02d", tUTC.Hour(), tUTC.Minute())
	fullDate := fmt.Sprintf("%d %s %d · %s", tUTC.Day(), russianMonth(tUTC.Month()), tUTC.Year(), timeStr)

	switch {
	case diff < 0:
		when = fullDate
		whenSub = "прошедшее"
	case diff < time.Hour:
		mins := int(diff.Minutes())
		when = fmt.Sprintf("Сегодня · %s", timeStr)
		whenSub = fmt.Sprintf("через %d мин", mins)
	case diff < 24*time.Hour:
		when = fmt.Sprintf("Сегодня · %s", timeStr)
		whenSub = fmt.Sprintf("через %d ч", int(diff.Hours()))
	case days == 1:
		when = fmt.Sprintf("Завтра · %s", timeStr)
		whenSub = fmt.Sprintf("%d %s", tUTC.Day(), russianMonth(tUTC.Month()))
	default:
		when = fullDate
		whenSub = fmt.Sprintf("через %d %s", days, pluralDays(days))
	}
	return
}

func pluralDays(n int) string {
	if n%10 == 1 && n%100 != 11 {
		return "день"
	}
	if n%10 >= 2 && n%10 <= 4 && (n%100 < 10 || n%100 >= 20) {
		return "дня"
	}
	return "дней"
}

func russianWeekday(d time.Weekday) string {
	return [...]string{"Вс", "Пн", "Вт", "Ср", "Чт", "Пт", "Сб"}[d]
}

func russianMonth(m time.Month) string {
	return [...]string{"", "янв", "фев", "мар", "апр", "май", "июн",
		"июл", "авг", "сен", "окт", "ноя", "дек"}[m]
}

// ─── Request flow ─────────────────────────────────────────────────

// SubmitRequest inserts a new join request.
// If a rejected request already exists, it resets it to pending (re-apply).
// Returns ErrAlreadyJoined if user is already a member.
// Returns ErrAlreadyRequested if a pending request already exists.
func (r *SborRepository) SubmitRequest(ctx context.Context, sborID, userID, message string) error {
	// Check already member
	var isMember bool
	_ = r.db.Pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM sbor_members WHERE sbor_id=$1 AND user_id=$2)`,
		sborID, userID).Scan(&isMember)
	if isMember {
		return domain.ErrAlreadyJoined
	}

	tag, err := r.db.Pool.Exec(ctx, `
		INSERT INTO sbor_requests (sbor_id, user_id, message)
		VALUES ($1, $2, $3)
		ON CONFLICT (sbor_id, user_id) DO UPDATE
		    SET status     = 'pending',
		        message    = EXCLUDED.message,
		        updated_at = now()
		WHERE sbor_requests.status = 'rejected'
	`, sborID, userID, message)
	if err != nil {
		return err
	}
	// If 0 rows affected the conflict row already has status='pending'
	if tag.RowsAffected() == 0 {
		return domain.ErrAlreadyRequested
	}
	return nil
}

// CancelRequest removes a pending request. User can only cancel their own.
func (r *SborRepository) CancelRequest(ctx context.Context, sborID, userID string) error {
	tag, err := r.db.Pool.Exec(ctx,
		`DELETE FROM sbor_requests WHERE sbor_id=$1 AND user_id=$2 AND status='pending'`,
		sborID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrRequestNotFound
	}
	return nil
}

// ApproveRequest atomically checks slots, adds user to sbor_members, marks request approved.
// Returns the approved user's ID so the caller can add them to the group chat.
func (r *SborRepository) ApproveRequest(ctx context.Context, requestID, adminID string) (approvedUserID string, sborID string, err error) {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return "", "", err
	}
	defer tx.Rollback(ctx)

	var maxSlots *int
	err = tx.QueryRow(ctx, `
		SELECT sr.user_id, sr.sbor_id, s.max_slots
		FROM sbor_requests sr
		JOIN sbory s ON s.id = sr.sbor_id
		WHERE sr.id = $1 AND sr.status = 'pending' AND s.host_id = $2
		FOR UPDATE OF s
	`, requestID, adminID).Scan(&approvedUserID, &sborID, &maxSlots)
	if err == pgx.ErrNoRows {
		return "", "", domain.ErrRequestNotFound
	}
	if err != nil {
		return "", "", err
	}

	// Slot check
	if maxSlots != nil {
		var count int
		_ = tx.QueryRow(ctx, `SELECT COUNT(*) FROM sbor_members WHERE sbor_id=$1`, sborID).Scan(&count)
		if count >= *maxSlots {
			return "", "", domain.ErrSborFull
		}
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO sbor_members (sbor_id, user_id, role) VALUES ($1, $2, 'participant')
		ON CONFLICT DO NOTHING
	`, sborID, approvedUserID)
	if err != nil {
		return "", "", err
	}

	_, err = tx.Exec(ctx,
		`UPDATE sbor_requests SET status='approved', updated_at=now() WHERE id=$1`,
		requestID)
	if err != nil {
		return "", "", err
	}

	return approvedUserID, sborID, tx.Commit(ctx)
}

// RejectRequest marks a pending request as rejected. Returns the user ID.
func (r *SborRepository) RejectRequest(ctx context.Context, requestID, adminID string) (rejectedUserID string, sborID string, err error) {
	err = r.db.Pool.QueryRow(ctx, `
		UPDATE sbor_requests sr
		SET status = 'rejected', updated_at = now()
		FROM sbory s
		WHERE sr.id = $1 AND sr.sbor_id = s.id AND s.host_id = $2 AND sr.status = 'pending'
		RETURNING sr.user_id, sr.sbor_id
	`, requestID, adminID).Scan(&rejectedUserID, &sborID)
	if err == pgx.ErrNoRows {
		return "", "", domain.ErrRequestNotFound
	}
	return rejectedUserID, sborID, err
}

// ListRequests returns pending join requests for a sbor. adminID must be the host.
func (r *SborRepository) ListRequests(ctx context.Context, sborID, adminID string) ([]*domain.SborJoinRequest, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT sr.id, sr.sbor_id, sr.user_id, sr.status, sr.message, sr.created_at,
		       u.full_name, COALESCE(u.username,''), COALESCE(u.avatar_url,'')
		FROM sbor_requests sr
		JOIN users u ON u.id = sr.user_id
		JOIN sbory s ON s.id = sr.sbor_id
		WHERE sr.sbor_id = $1 AND s.host_id = $2 AND sr.status = 'pending'
		ORDER BY sr.created_at ASC
	`, sborID, adminID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*domain.SborJoinRequest
	for rows.Next() {
		req := &domain.SborJoinRequest{}
		if err := rows.Scan(
			&req.ID, &req.SborID, &req.UserID, &req.Status, &req.Message, &req.CreatedAt,
			&req.FullName, &req.Username, &req.AvatarURL,
		); err != nil {
			return nil, err
		}
		result = append(result, req)
	}
	return result, rows.Err()
}

// ─── Bookmarks ────────────────────────────────────────────────────

// ToggleBookmark добавляет или убирает сбор из закладок пользователя.
// Idempotent: повторный вызов для уже-сохранённого — удаляет, для
// не-сохранённого — добавляет. Возвращает новое состояние (true = сохранено).
func (r *SborRepository) ToggleBookmark(ctx context.Context, userID, sborID string) (bool, error) {
	tag, err := r.db.Pool.Exec(ctx,
		`DELETE FROM sbor_bookmarks WHERE user_id=$1 AND sbor_id=$2`,
		userID, sborID)
	if err != nil {
		return false, fmt.Errorf("toggle bookmark delete: %w", err)
	}
	if tag.RowsAffected() > 0 {
		return false, nil // было — удалили
	}
	_, err = r.db.Pool.Exec(ctx,
		`INSERT INTO sbor_bookmarks (user_id, sbor_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`,
		userID, sborID)
	if err != nil {
		return false, fmt.Errorf("toggle bookmark insert: %w", err)
	}
	return true, nil
}

// IsBookmarked возвращает true если пользователь сохранил этот сбор.
func (r *SborRepository) IsBookmarked(ctx context.Context, userID, sborID string) (bool, error) {
	var exists bool
	err := r.db.Pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM sbor_bookmarks WHERE user_id=$1 AND sbor_id=$2)`,
		userID, sborID).Scan(&exists)
	return exists, err
}

// ListBookmarked возвращает список сборов в закладках пользователя.
func (r *SborRepository) ListBookmarked(ctx context.Context, userID string, limit, offset int) ([]*domain.Sbor, error) {
	// Single query: join bookmarks → sbory — same SELECT shape as List so we
	// can reuse the batch member-load below. Replaces the previous N+1 pattern
	// (one GetByID call per bookmarked sbor).
	rows, err := r.db.Pool.Query(ctx, `
		SELECT s.id, s.host_id, s.type, s.category, s.title, s.place, s.city,
		       s.cover_url, s.price, s.description, s.scheduled_at, s.flexible_time, s.max_slots,
		       s.is_live, s.created_at, s.chat_id,
		       u.full_name,
		       COUNT(DISTINCT sm.user_id)::int AS joined,
		       COALESCE(
		         (SELECT sm2.role FROM sbor_members sm2
		          WHERE sm2.sbor_id = s.id AND sm2.user_id = $1), ''
		       ) AS my_role,
		       EXISTS(
		         SELECT 1 FROM sbor_members sm3
		         WHERE sm3.sbor_id = s.id AND sm3.user_id = $1
		       ) AS is_joined,
		       true AS is_bookmarked,
		       COALESCE(
		         (SELECT rq.status FROM sbor_requests rq
		          WHERE rq.sbor_id = s.id AND rq.user_id = $1), ''
		       ) AS my_request_status,
		       (SELECT COUNT(*)::int FROM sbor_requests rq2
		        WHERE rq2.sbor_id = s.id AND rq2.status = 'pending') AS pending_requests_count
		FROM sbor_bookmarks bk
		JOIN sbory s ON s.id = bk.sbor_id AND NOT s.is_cancelled
		JOIN users u ON u.id = s.host_id
		LEFT JOIN sbor_members sm ON sm.sbor_id = s.id
		WHERE bk.user_id = $1
		GROUP BY s.id, u.full_name, bk.created_at
		ORDER BY bk.created_at DESC
		LIMIT $2 OFFSET $3`, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list bookmarked: %w", err)
	}
	defer rows.Close()

	var result []*domain.Sbor
	idxByID := make(map[string]int)
	for rows.Next() {
		s := &domain.Sbor{}
		if err := rows.Scan(
			&s.ID, &s.HostID, &s.Type, &s.Category, &s.Title, &s.Place, &s.City,
			&s.CoverUrl, &s.Price, &s.Description, &s.ScheduledAt, &s.FlexibleTime, &s.MaxSlots,
			&s.IsLive, &s.CreatedAt, &s.ChatID,
			&s.HostName, &s.Joined, &s.MyRole, &s.IsJoined, &s.IsBookmarked,
			&s.MyRequestStatus, &s.PendingRequestsCount,
		); err != nil {
			return nil, err
		}
		s.When, s.WhenSub = formatWhen(s.ScheduledAt, s.FlexibleTime)
		idxByID[s.ID] = len(result)
		result = append(result, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return result, nil
	}

	// Batch-load member previews (one query for all sbors).
	ids := make([]string, len(result))
	for i, s := range result {
		ids[i] = s.ID
	}
	memberRows, err := r.db.Pool.Query(ctx, `
		SELECT sm.sbor_id, u.full_name, COALESCE(u.username,''), u.id, COALESCE(u.avatar_url,'')
		FROM sbor_members sm
		JOIN users u ON u.id = sm.user_id
		WHERE sm.sbor_id = ANY($1)
		ORDER BY sm.sbor_id, sm.joined_at`, ids)
	if err == nil {
		defer memberRows.Close()
		counters := make(map[string]int, len(result))
		for memberRows.Next() {
			var sborID, name, username, uid, avatarURL string
			if err := memberRows.Scan(&sborID, &name, &username, &uid, &avatarURL); err != nil {
				continue
			}
			if counters[sborID] >= 8 {
				continue
			}
			counters[sborID]++
			i := idxByID[sborID]
			result[i].MemberNames = append(result[i].MemberNames, name)
			result[i].MemberUsernames = append(result[i].MemberUsernames, username)
			result[i].MemberIDs = append(result[i].MemberIDs, uid)
			result[i].MemberAvatarURLs = append(result[i].MemberAvatarURLs, avatarURL)
		}
	}
	return result, nil
}
