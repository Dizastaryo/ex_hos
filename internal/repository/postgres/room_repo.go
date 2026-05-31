package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/seeu/backend/internal/domain"
)

type RoomRepository struct {
	db *DB
}

func NewRoomRepository(db *DB) *RoomRepository {
	return &RoomRepository{db: db}
}

// ─── Create ──────────────────────────────────────────────────────

func (r *RoomRepository) Create(ctx context.Context, req *domain.CreateRoomRequest, creatorID string) (*domain.Room, error) {
	room := &domain.Room{}
	err := r.db.Pool.QueryRow(ctx, `
		INSERT INTO rooms (creator_id, type, name, description, cover_url, is_public)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING id, creator_id, type, name, description, cover_url, is_public, is_active, created_at`,
		creatorID, req.Type, req.Name, req.Description, req.CoverURL, req.IsPublic,
	).Scan(
		&room.ID, &room.CreatorID, &room.Type, &room.Name,
		&room.Description, &room.CoverURL, &room.IsPublic, &room.IsActive, &room.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create room: %w", err)
	}
	// Creator auto-joins and is always admin
	_, err = r.db.Pool.Exec(ctx,
		`INSERT INTO room_participants (room_id, user_id, is_admin) VALUES ($1,$2,true) ON CONFLICT DO NOTHING`,
		room.ID, creatorID,
	)
	if err != nil {
		return nil, fmt.Errorf("auto-join creator: %w", err)
	}
	room.IsJoined = true
	room.IsAdmin = true // creator is always admin
	room.ParticipantCount = 1
	return room, nil
}

// ─── GetByID ─────────────────────────────────────────────────────

func (r *RoomRepository) GetByID(ctx context.Context, id, viewerID string) (*domain.Room, error) {
	room := &domain.Room{}
	err := r.db.Pool.QueryRow(ctx, `
		SELECT r.id, r.creator_id, r.type, r.name, r.description, r.cover_url,
		       r.is_public, r.is_active, r.created_at,
		       u.full_name AS creator_name,
		       COUNT(DISTINCT rp.user_id)::int AS participant_count,
		       EXISTS(SELECT 1 FROM room_participants WHERE room_id=r.id AND user_id=$2) AS is_joined,
		       COALESCE(
		           (SELECT is_muted FROM room_participants
		            WHERE room_id=r.id AND user_id=$2), false
		       ) AS is_muted,
		       COALESCE(
		           (SELECT is_admin FROM room_participants
		            WHERE room_id=r.id AND user_id=$2), false
		       ) AS is_admin
		FROM rooms r
		JOIN users u ON u.id = r.creator_id
		LEFT JOIN room_participants rp ON rp.room_id = r.id
		WHERE r.id=$1
		GROUP BY r.id, u.full_name`,
		id, viewerID,
	).Scan(
		&room.ID, &room.CreatorID, &room.Type, &room.Name, &room.Description, &room.CoverURL,
		&room.IsPublic, &room.IsActive, &room.CreatedAt,
		&room.CreatorName, &room.ParticipantCount, &room.IsJoined, &room.IsMuted, &room.IsAdmin,
	)
	if err == pgx.ErrNoRows {
		return nil, domain.ErrRoomNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get room: %w", err)
	}
	// Private rooms are only accessible to members.
	if !room.IsPublic && !room.IsJoined {
		return nil, domain.ErrForbidden
	}
	room.Participants = r.getParticipants(ctx, id)
	room.LastMessage, room.LastMessageAt = r.getLastMessage(ctx, id)
	return room, nil
}

// ─── List ─────────────────────────────────────────────────────────

func (r *RoomRepository) List(ctx context.Context, viewerID string, limit, offset int) ([]*domain.Room, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT r.id, r.creator_id, r.type, r.name, r.description, r.cover_url,
		       r.is_public, r.is_active, r.created_at,
		       u.full_name AS creator_name,
		       COUNT(DISTINCT rp.user_id)::int AS participant_count,
		       EXISTS(SELECT 1 FROM room_participants WHERE room_id=r.id AND user_id=$1) AS is_joined
		FROM rooms r
		JOIN users u ON u.id = r.creator_id
		LEFT JOIN room_participants rp ON rp.room_id = r.id
		WHERE r.is_active
		  AND (r.is_public OR EXISTS(
		           SELECT 1 FROM room_participants
		           WHERE room_id = r.id AND user_id = $1
		       ))
		GROUP BY r.id, u.full_name
		ORDER BY participant_count DESC, r.created_at DESC
		LIMIT $2 OFFSET $3`,
		viewerID, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("list rooms: %w", err)
	}
	defer rows.Close()

	var result []*domain.Room
	for rows.Next() {
		room := &domain.Room{}
		if err := rows.Scan(
			&room.ID, &room.CreatorID, &room.Type, &room.Name, &room.Description, &room.CoverURL,
			&room.IsPublic, &room.IsActive, &room.CreatedAt,
			&room.CreatorName, &room.ParticipantCount, &room.IsJoined,
		); err != nil {
			return nil, err
		}
		// Load a few participants for preview (avatar stack in UI)
		room.Participants = r.getParticipantsLimit(ctx, room.ID, 5)
		room.LastMessage, room.LastMessageAt = r.getLastMessage(ctx, room.ID)
		result = append(result, room)
	}
	return result, nil
}

// ─── Join / Leave ─────────────────────────────────────────────────

// Join adds a user to a room. For private rooms it is blocked — use InviteMember instead.
func (r *RoomRepository) Join(ctx context.Context, roomID, userID string) error {
	var isPublic bool
	err := r.db.Pool.QueryRow(ctx,
		`SELECT is_public FROM rooms WHERE id=$1`, roomID,
	).Scan(&isPublic)
	if err == pgx.ErrNoRows {
		return domain.ErrRoomNotFound
	}
	if err != nil {
		return fmt.Errorf("join room check: %w", err)
	}
	if !isPublic {
		return domain.ErrPrivateRoom
	}
	_, err = r.db.Pool.Exec(ctx,
		`INSERT INTO room_participants (room_id, user_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`,
		roomID, userID,
	)
	return err
}

// InviteMember adds a user to a private room. Creator or any admin may do this.
func (r *RoomRepository) InviteMember(ctx context.Context, roomID, inviterID, userID string) error {
	if !r.isAdminOrCreator(ctx, roomID, inviterID) {
		// Check room exists first
		var exists bool
		_ = r.db.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM rooms WHERE id=$1)`, roomID).Scan(&exists)
		if !exists {
			return domain.ErrRoomNotFound
		}
		return domain.ErrForbidden
	}
	_, err := r.db.Pool.Exec(ctx,
		`INSERT INTO room_participants (room_id, user_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`,
		roomID, userID,
	)
	return err
}

// RemoveMember removes a member from a room. Creator or any admin may do this,
// but admins cannot remove the creator or other admins (only creator can).
func (r *RoomRepository) RemoveMember(ctx context.Context, roomID, requesterID, targetID string) error {
	var creatorID string
	err := r.db.Pool.QueryRow(ctx,
		`SELECT creator_id FROM rooms WHERE id=$1`, roomID,
	).Scan(&creatorID)
	if err == pgx.ErrNoRows {
		return domain.ErrRoomNotFound
	}
	if err != nil {
		return fmt.Errorf("remove member check: %w", err)
	}
	if targetID == creatorID {
		return domain.ErrForbidden // nobody can remove the creator
	}
	isCreator := requesterID == creatorID
	if !isCreator {
		// Admins can remove regular members only
		if !r.isAdminOrCreator(ctx, roomID, requesterID) {
			return domain.ErrForbidden
		}
		// Admins cannot remove other admins
		var targetIsAdmin bool
		_ = r.db.Pool.QueryRow(ctx,
			`SELECT is_admin FROM room_participants WHERE room_id=$1 AND user_id=$2`, roomID, targetID,
		).Scan(&targetIsAdmin)
		if targetIsAdmin {
			return domain.ErrForbidden
		}
	}
	_, err = r.db.Pool.Exec(ctx,
		`DELETE FROM room_participants WHERE room_id=$1 AND user_id=$2`,
		roomID, targetID,
	)
	return err
}

// ListMembers returns all members of a room. The caller must already be a member.
func (r *RoomRepository) ListMembers(ctx context.Context, roomID, viewerID string) ([]domain.RoomMember, error) {
	var isMember bool
	_ = r.db.Pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM room_participants WHERE room_id=$1 AND user_id=$2)`,
		roomID, viewerID,
	).Scan(&isMember)
	if !isMember {
		return nil, domain.ErrForbidden
	}
	var creatorID string
	_ = r.db.Pool.QueryRow(ctx,
		`SELECT creator_id FROM rooms WHERE id=$1`, roomID,
	).Scan(&creatorID)

	rows, err := r.db.Pool.Query(ctx, `
		SELECT rp.user_id, u.full_name, u.username, COALESCE(u.avatar_url,''), rp.is_muted, rp.is_admin, rp.joined_at
		FROM room_participants rp
		JOIN users u ON u.id = rp.user_id
		WHERE rp.room_id = $1
		ORDER BY rp.joined_at ASC`,
		roomID,
	)
	if err != nil {
		return nil, fmt.Errorf("list members: %w", err)
	}
	defer rows.Close()

	var out []domain.RoomMember
	for rows.Next() {
		m := domain.RoomMember{}
		if err := rows.Scan(&m.UserID, &m.FullName, &m.Username, &m.AvatarURL, &m.IsMuted, &m.IsAdmin, &m.JoinedAt); err != nil {
			return nil, err
		}
		m.IsCreator = m.UserID == creatorID
		out = append(out, m)
	}
	return out, nil
}

func (r *RoomRepository) Leave(ctx context.Context, roomID, userID string) error {
	// Creator cannot leave their own room — they must close it instead.
	var creatorID string
	err := r.db.Pool.QueryRow(ctx, `SELECT creator_id FROM rooms WHERE id=$1`, roomID).Scan(&creatorID)
	if err == pgx.ErrNoRows {
		return domain.ErrRoomNotFound
	}
	if err != nil {
		return fmt.Errorf("leave room check: %w", err)
	}
	if creatorID == userID {
		return domain.ErrForbidden
	}
	_, err = r.db.Pool.Exec(ctx,
		`DELETE FROM room_participants WHERE room_id=$1 AND user_id=$2`,
		roomID, userID,
	)
	return err
}

// ─── Mute ─────────────────────────────────────────────────────────

func (r *RoomRepository) SetMuted(ctx context.Context, roomID, userID string, muted bool) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE room_participants SET is_muted=$3 WHERE room_id=$1 AND user_id=$2`,
		roomID, userID, muted,
	)
	return err
}

func (r *RoomRepository) IsMuted(ctx context.Context, roomID, userID string) (bool, error) {
	var muted bool
	err := r.db.Pool.QueryRow(ctx,
		`SELECT is_muted FROM room_participants WHERE room_id=$1 AND user_id=$2`,
		roomID, userID,
	).Scan(&muted)
	if err == pgx.ErrNoRows {
		return false, domain.ErrNotInRoom
	}
	return muted, err
}

// ─── Update ───────────────────────────────────────────────────────

// UpdateRoom updates name/description/cover. Only admins (or creator) may call this.
func (r *RoomRepository) UpdateRoom(ctx context.Context, roomID, requesterID string, req *domain.UpdateRoomRequest) error {
	if !r.isAdminOrCreator(ctx, roomID, requesterID) {
		var exists bool
		_ = r.db.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM rooms WHERE id=$1)`, roomID).Scan(&exists)
		if !exists {
			return domain.ErrRoomNotFound
		}
		return domain.ErrForbidden
	}
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE rooms SET name=$2, description=$3, cover_url=$4, updated_at=now() WHERE id=$1`,
		roomID, req.Name, req.Description, req.CoverURL,
	)
	return err
}

// SetAdmin grants or revokes admin status. Only the creator may do this.
func (r *RoomRepository) SetAdmin(ctx context.Context, roomID, requesterID, targetID string, grant bool) error {
	var creatorID string
	err := r.db.Pool.QueryRow(ctx, `SELECT creator_id FROM rooms WHERE id=$1`, roomID).Scan(&creatorID)
	if err == pgx.ErrNoRows {
		return domain.ErrRoomNotFound
	}
	if err != nil {
		return fmt.Errorf("set admin check: %w", err)
	}
	if creatorID != requesterID {
		return domain.ErrForbidden
	}
	if targetID == creatorID {
		return nil // creator is always admin, no-op
	}
	// Target must be a member
	var isMember bool
	_ = r.db.Pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM room_participants WHERE room_id=$1 AND user_id=$2)`, roomID, targetID,
	).Scan(&isMember)
	if !isMember {
		return domain.ErrNotInRoom
	}
	_, err = r.db.Pool.Exec(ctx,
		`UPDATE room_participants SET is_admin=$3 WHERE room_id=$1 AND user_id=$2`,
		roomID, targetID, grant,
	)
	return err
}

// isAdminOrCreator returns true if the user has admin role or is the room creator.
func (r *RoomRepository) isAdminOrCreator(ctx context.Context, roomID, userID string) bool {
	var isAdmin bool
	err := r.db.Pool.QueryRow(ctx,
		`SELECT is_admin FROM room_participants WHERE room_id=$1 AND user_id=$2`, roomID, userID,
	).Scan(&isAdmin)
	if err != nil {
		return false
	}
	return isAdmin
}

// ─── Close ────────────────────────────────────────────────────────

func (r *RoomRepository) Close(ctx context.Context, roomID string) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE rooms SET is_active=false, updated_at=now() WHERE id=$1`,
		roomID,
	)
	return err
}

// ─── Messages ─────────────────────────────────────────────────────

func (r *RoomRepository) SendMessage(ctx context.Context, roomID, senderID, text string) (*domain.RoomMessage, error) {
	msg := &domain.RoomMessage{}
	err := r.db.Pool.QueryRow(ctx, `
		INSERT INTO room_messages (room_id, sender_id, text)
		VALUES ($1,$2,$3)
		RETURNING id, room_id, sender_id, text, created_at`,
		roomID, senderID, text,
	).Scan(&msg.ID, &msg.RoomID, &msg.SenderID, &msg.Text, &msg.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("send room message: %w", err)
	}
	// Load sender info
	_ = r.db.Pool.QueryRow(ctx,
		`SELECT full_name, username, COALESCE(avatar_url,'') FROM users WHERE id=$1`,
		senderID,
	).Scan(&msg.SenderName, &msg.SenderUsername, &msg.SenderAvatar)
	return msg, nil
}

func (r *RoomRepository) GetMessages(ctx context.Context, roomID string, limit, offset int) ([]*domain.RoomMessage, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT m.id, m.room_id, m.sender_id, m.text, m.created_at,
		       u.full_name, u.username, COALESCE(u.avatar_url,'')
		FROM room_messages m
		JOIN users u ON u.id = m.sender_id
		WHERE m.room_id=$1
		ORDER BY m.created_at DESC
		LIMIT $2 OFFSET $3`,
		roomID, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("get room messages: %w", err)
	}
	defer rows.Close()

	var result []*domain.RoomMessage
	for rows.Next() {
		msg := &domain.RoomMessage{}
		if err := rows.Scan(
			&msg.ID, &msg.RoomID, &msg.SenderID, &msg.Text, &msg.CreatedAt,
			&msg.SenderName, &msg.SenderUsername, &msg.SenderAvatar,
		); err != nil {
			return nil, err
		}
		result = append(result, msg)
	}
	// Reverse so oldest first
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}
	return result, nil
}

// ─── Helpers ──────────────────────────────────────────────────────

func (r *RoomRepository) GetParticipantIDs(ctx context.Context, roomID string) ([]string, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT user_id FROM room_participants WHERE room_id=$1`, roomID)
	if err != nil {
		return nil, err
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
	return ids, nil
}

func (r *RoomRepository) getParticipants(ctx context.Context, roomID string) []domain.RoomParticipant {
	return r.getParticipantsLimit(ctx, roomID, 50)
}

func (r *RoomRepository) getParticipantsLimit(ctx context.Context, roomID string, limit int) []domain.RoomParticipant {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT rp.user_id, u.full_name, u.username, COALESCE(u.avatar_url,''), rp.is_muted
		FROM room_participants rp
		JOIN users u ON u.id = rp.user_id
		WHERE rp.room_id=$1
		ORDER BY rp.joined_at
		LIMIT $2`,
		roomID, limit,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []domain.RoomParticipant
	for rows.Next() {
		p := domain.RoomParticipant{}
		if err := rows.Scan(&p.UserID, &p.FullName, &p.Username, &p.AvatarURL, &p.IsMuted); err == nil {
			out = append(out, p)
		}
	}
	return out
}

func (r *RoomRepository) getLastMessage(ctx context.Context, roomID string) (string, *time.Time) {
	var text string
	var at time.Time
	err := r.db.Pool.QueryRow(ctx,
		`SELECT text, created_at FROM room_messages WHERE room_id=$1 ORDER BY created_at DESC LIMIT 1`,
		roomID,
	).Scan(&text, &at)
	if err != nil {
		return "", nil
	}
	return text, &at
}
