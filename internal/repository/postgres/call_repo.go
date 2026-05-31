package postgres

import (
	"context"
	"fmt"
	"time"
)

// Call — record в `call_invitations` table (C-1 история звонков).
type Call struct {
	ID              string     `json:"id"`
	FromUserID      string     `json:"from_user_id"`
	ToUserID        string     `json:"to_user_id"`
	Kind            string     `json:"kind"`   // 'video' | 'voice'
	Status          string     `json:"status"` // pending/accepted/declined/missed/ended
	StartedAt       time.Time  `json:"started_at"`
	AcceptedAt      *time.Time `json:"accepted_at,omitempty"`
	EndedAt         *time.Time `json:"ended_at,omitempty"`
	DurationSeconds *int       `json:"duration_seconds,omitempty"`
	// Hydrated полями peer'а — фронт-чтобы не делать N+1 на /me/calls.
	PeerUsername  string `json:"peer_username,omitempty"`
	PeerFullName  string `json:"peer_full_name,omitempty"`
	PeerAvatarURL string `json:"peer_avatar_url,omitempty"`
	// «Кто я в этом звонке» — incoming/outgoing — computed на фронте по
	// from_user_id == me. Бэк не пишет (анти-дублирование).
}

type CallRepository struct {
	db *DB
}

func NewCallRepository(db *DB) *CallRepository {
	return &CallRepository{db: db}
}

// CreateInvite вставляет новый call_invitation в статусе pending. Вызывается
// из ws_handler.relayCallEvent при `call.invite`. Возвращает id записи —
// если когда-то понадобится корреляция, сейчас не используется.
func (r *CallRepository) CreateInvite(
	ctx context.Context, fromID, toID, kind string,
) (string, error) {
	if kind != "voice" && kind != "video" {
		kind = "video"
	}
	var id string
	err := r.db.Pool.QueryRow(ctx, `
		INSERT INTO call_invitations (from_user_id, to_user_id, kind, status)
		VALUES ($1, $2, $3, 'pending')
		RETURNING id`,
		fromID, toID, kind,
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("create invite: %w", err)
	}
	return id, nil
}

// MarkAccepted переводит latest pending pair (from→to) в accepted.
// Если pending'а нет — no-op (звонок мог быть уже decline'нут).
func (r *CallRepository) MarkAccepted(
	ctx context.Context, fromID, toID string,
) error {
	_, err := r.db.Pool.Exec(ctx, `
		UPDATE call_invitations
		SET status = 'accepted', accepted_at = NOW()
		WHERE id = (
			SELECT id FROM call_invitations
			WHERE from_user_id = $1 AND to_user_id = $2 AND status = 'pending'
			ORDER BY started_at DESC LIMIT 1
		)`,
		fromID, toID,
	)
	if err != nil {
		return fmt.Errorf("mark accepted: %w", err)
	}
	return nil
}

// MarkDeclined — peer отказался от приглашения. То же matching что и Accepted.
func (r *CallRepository) MarkDeclined(
	ctx context.Context, fromID, toID string,
) error {
	_, err := r.db.Pool.Exec(ctx, `
		UPDATE call_invitations
		SET status = 'declined', ended_at = NOW()
		WHERE id = (
			SELECT id FROM call_invitations
			WHERE from_user_id = $1 AND to_user_id = $2 AND status = 'pending'
			ORDER BY started_at DESC LIMIT 1
		)`,
		fromID, toID,
	)
	if err != nil {
		return fmt.Errorf("mark declined: %w", err)
	}
	return nil
}

// MarkEnded закрывает active call. Возвращает (callerID, calleeID, wasMissed):
//   - wasMissed = true если status был pending (caller сбросил пока callee
//     не ответил) — caller должен показать «missed call» нотификацию callee.
//   - Если accepted → duration_seconds считаем от accepted_at.
//   - Если уже ended/declined/missed → no-op (idempotent).
//
// callerID может быть либо `from` либо `to` — мы не знаем кто из них вызвал
// `call.end` (любой может). Поэтому ищем latest active row для пары (любая
// direction).
func (r *CallRepository) MarkEnded(
	ctx context.Context, userA, userB string,
) (callerID, calleeID string, wasMissed bool, err error) {
	// Ищем latest call между парой (любая direction) в активном статусе.
	var (
		id        string
		fromID    string
		toID      string
		status    string
		accepted  *time.Time
	)
	err = r.db.Pool.QueryRow(ctx, `
		SELECT id, from_user_id, to_user_id, status, accepted_at
		FROM call_invitations
		WHERE status IN ('pending','accepted')
		  AND ((from_user_id = $1 AND to_user_id = $2)
		       OR (from_user_id = $2 AND to_user_id = $1))
		ORDER BY started_at DESC LIMIT 1`,
		userA, userB,
	).Scan(&id, &fromID, &toID, &status, &accepted)
	if err != nil {
		// No active row — no-op.
		return "", "", false, nil
	}
	newStatus := "ended"
	missed := false
	if status == "pending" {
		newStatus = "missed"
		missed = true
	}
	var duration *int
	if status == "accepted" && accepted != nil {
		d := int(time.Since(*accepted).Seconds())
		if d < 0 {
			d = 0
		}
		duration = &d
	}
	if _, err := r.db.Pool.Exec(ctx, `
		UPDATE call_invitations
		SET status = $1, ended_at = NOW(), duration_seconds = $2
		WHERE id = $3`,
		newStatus, duration, id,
	); err != nil {
		return "", "", false, fmt.Errorf("mark ended: %w", err)
	}
	return fromID, toID, missed, nil
}

// GetPendingFor (BUG-5) — возвращает active pending invitations адресованные
// userID. Используется фронтом при reconnect'е чтобы догнать звонок который
// пришёл когда WS был down. Pending = status='pending', не старше 60 секунд
// (стандартный ring-timeout). Старее — caller уже сдался / missed-notif.
func (r *CallRepository) GetPendingFor(
	ctx context.Context, userID string,
) ([]Call, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT c.id, c.from_user_id, c.to_user_id, c.kind, c.status,
		       c.started_at, c.accepted_at, c.ended_at, c.duration_seconds,
		       peer.username, peer.full_name, peer.avatar_url
		FROM call_invitations c
		JOIN users peer ON peer.id = c.from_user_id
		WHERE c.to_user_id = $1
		  AND c.status = 'pending'
		  AND c.started_at > NOW() - INTERVAL '60 seconds'
		ORDER BY c.started_at DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("get pending for user: %w", err)
	}
	defer rows.Close()
	var out []Call
	for rows.Next() {
		var c Call
		var avatar *string
		if err := rows.Scan(
			&c.ID, &c.FromUserID, &c.ToUserID, &c.Kind, &c.Status,
			&c.StartedAt, &c.AcceptedAt, &c.EndedAt, &c.DurationSeconds,
			&c.PeerUsername, &c.PeerFullName, &avatar,
		); err != nil {
			return nil, fmt.Errorf("scan call: %w", err)
		}
		if avatar != nil {
			c.PeerAvatarURL = *avatar
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// GetForUser возвращает list звонков где userID был caller или callee.
// Свежие сверху. Hydrate peer-fields (username/fullname/avatar) — для фронта
// без N+1.
func (r *CallRepository) GetForUser(
	ctx context.Context, userID string, limit, offset int,
) ([]Call, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT c.id, c.from_user_id, c.to_user_id, c.kind, c.status,
		       c.started_at, c.accepted_at, c.ended_at, c.duration_seconds,
		       peer.username, peer.full_name, peer.avatar_url
		FROM call_invitations c
		JOIN users peer ON peer.id = CASE
			WHEN c.from_user_id = $1 THEN c.to_user_id
			ELSE c.from_user_id
		END
		WHERE c.from_user_id = $1 OR c.to_user_id = $1
		ORDER BY c.started_at DESC
		LIMIT $2 OFFSET $3`,
		userID, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("get for user: %w", err)
	}
	defer rows.Close()
	var out []Call
	for rows.Next() {
		var c Call
		var avatar *string
		if err := rows.Scan(
			&c.ID, &c.FromUserID, &c.ToUserID, &c.Kind, &c.Status,
			&c.StartedAt, &c.AcceptedAt, &c.EndedAt, &c.DurationSeconds,
			&c.PeerUsername, &c.PeerFullName, &avatar,
		); err != nil {
			return nil, fmt.Errorf("scan call: %w", err)
		}
		if avatar != nil {
			c.PeerAvatarURL = *avatar
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
