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
		SELECT s.id, s.host_id, s.type, s.category, s.title, s.place,
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
		&s.ID, &s.HostID, &s.Type, &s.Category, &s.Title, &s.Place,
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

	s.MemberNames, s.MemberUsernames, s.MemberIDs, _ = r.getMemberUsers(ctx, id)
	s.When, s.WhenSub = formatWhen(s.ScheduledAt, s.FlexibleTime)
	return s, nil
}

// ─── List (feed) ──────────────────────────────────────────────────

func (r *SborRepository) List(ctx context.Context, viewerID, typeFilter, catFilter, cityFilter string, limit, offset int) ([]*domain.Sbor, error) {
	args := []any{viewerID, limit, offset}
	where := "NOT s.is_cancelled"
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
			SELECT sm.sbor_id, u.full_name, COALESCE(u.username, ''), u.id
			FROM sbor_members sm
			JOIN users u ON u.id = sm.user_id
			WHERE sm.sbor_id = ANY($1)
			ORDER BY sm.sbor_id, sm.joined_at
		`, ids)
		if err == nil {
			defer memberRows.Close()
			counters := make(map[string]int, len(result))
			for memberRows.Next() {
				var sborID, name, username, userID string
				if err := memberRows.Scan(&sborID, &name, &username, &userID); err != nil {
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
			}
		}
	}

	return result, nil
}

// ─── My sbory ────────────────────────────────────────────────────

func (r *SborRepository) ListMine(ctx context.Context, userID string, limit, offset int) ([]*domain.Sbor, error) {
	query := `
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
		WHERE NOT s.is_cancelled
		GROUP BY s.id, u.full_name
		ORDER BY s.created_at DESC
		LIMIT $2 OFFSET $3`

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
			SELECT sm.sbor_id, u.full_name, COALESCE(u.username, ''), u.id
			FROM sbor_members sm
			JOIN users u ON u.id = sm.user_id
			WHERE sm.sbor_id = ANY($1)
			ORDER BY sm.sbor_id, sm.joined_at
		`, ids)
		if err == nil {
			defer memberRows.Close()
			counters := make(map[string]int, len(result))
			for memberRows.Next() {
				var sborID, name, username, userID string
				if err := memberRows.Scan(&sborID, &name, &username, &userID); err != nil {
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
			}
		}
	}
	return result, nil
}

// ─── Join / Leave ────────────────────────────────────────────────

func (r *SborRepository) Join(ctx context.Context, sborID, userID string) error {
	_, err := r.db.Pool.Exec(ctx,
		`INSERT INTO sbor_members (sbor_id, user_id, role) VALUES ($1,$2,'participant')
		 ON CONFLICT DO NOTHING`,
		sborID, userID,
	)
	return err
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
		`UPDATE sbory SET title = $2, cover_url = $3, updated_at = now()
		 WHERE chat_id = $1 AND NOT is_cancelled`,
		chatID, title, coverURL,
	)
	return err
}

func (r *SborRepository) Cancel(ctx context.Context, id, hostID string) error {
	tag, err := r.db.Pool.Exec(ctx,
		`UPDATE sbory SET is_cancelled=true, updated_at=now() WHERE id=$1 AND host_id=$2`,
		id, hostID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrSborNotFound
	}
	return nil
}

// ─── Count helpers ────────────────────────────────────────────────

func (r *SborRepository) CountMembers(ctx context.Context, sborID string) (int, error) {
	var n int
	err := r.db.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM sbor_members WHERE sbor_id=$1`, sborID,
	).Scan(&n)
	return n, err
}

func (r *SborRepository) getMemberUsers(ctx context.Context, sborID string) (names, usernames, ids []string, err error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT u.full_name, COALESCE(u.username, ''), u.id
		FROM sbor_members sm
		JOIN users u ON u.id = sm.user_id
		WHERE sm.sbor_id = $1
		ORDER BY sm.joined_at
		LIMIT 8`, sborID)
	if err != nil {
		return nil, nil, nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var name, username, id string
		if err := rows.Scan(&name, &username, &id); err != nil {
			return nil, nil, nil, err
		}
		names = append(names, name)
		usernames = append(usernames, username)
		ids = append(ids, id)
	}
	return names, usernames, ids, nil
}

// ─── formatWhen ───────────────────────────────────────────────────

func formatWhen(t *time.Time, flexible bool) (when, whenSub string) {
	if flexible || t == nil {
		return "Гибко", "договоримся"
	}
	now := time.Now()
	diff := t.Sub(now)

	days := int(diff.Hours() / 24)
	weekday := russianWeekday(t.Weekday())
	timeStr := fmt.Sprintf("%02d:%02d", t.Hour(), t.Minute())

	switch {
	case diff < 0:
		when = fmt.Sprintf("%s · %s", weekday, timeStr)
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
		whenSub = "завтра"
	case days < 7:
		when = fmt.Sprintf("%s · %s", weekday, timeStr)
		whenSub = fmt.Sprintf("через %d %s", days, pluralDays(days))
	default:
		when = fmt.Sprintf("%s · %s", weekday, timeStr)
		whenSub = fmt.Sprintf("%d %s", t.Day(), russianMonth(t.Month()))
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
	rows, err := r.db.Pool.Query(ctx, `
		SELECT s.id FROM sbor_bookmarks sb
		JOIN sbory s ON s.id = sb.sbor_id AND NOT s.is_cancelled
		WHERE sb.user_id = $1
		ORDER BY sb.created_at DESC
		LIMIT $2 OFFSET $3`, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list bookmarked: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var result []*domain.Sbor
	for _, id := range ids {
		sbor, err := r.GetByID(ctx, id, userID)
		if err != nil {
			continue // сбор мог быть отменён между запросами
		}
		result = append(result, sbor)
	}
	return result, nil
}
