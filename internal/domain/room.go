package domain

import "time"

type Room struct {
	ID          string    `json:"id"`
	CreatorID   string    `json:"creator_id"`
	Type        string    `json:"type"` // "text" | "voice"
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	CoverURL    string    `json:"cover_url,omitempty"`
	IsPublic    bool      `json:"is_public"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`

	// Computed on load
	ParticipantCount int               `json:"participant_count"`
	Participants     []RoomParticipant `json:"participants,omitempty"`
	LastMessage      string            `json:"last_message,omitempty"`
	LastMessageAt    *time.Time        `json:"last_message_at,omitempty"`
	IsJoined         bool              `json:"is_joined"`
	IsMuted          bool              `json:"is_muted"`
	IsAdmin          bool              `json:"is_admin"`
	CreatorName      string            `json:"creator_name,omitempty"`
	// Voice channel (explicit opt-in, separate from room membership)
	VoiceCount        int               `json:"voice_count"`
	VoiceParticipants []RoomParticipant `json:"voice_participants,omitempty"`
	IsInVoice         bool              `json:"is_in_voice"`
}

type RoomParticipant struct {
	UserID    string `json:"user_id"`
	FullName  string `json:"full_name"`
	Username  string `json:"username"`
	AvatarURL string `json:"avatar_url,omitempty"`
	IsMuted   bool   `json:"is_muted"`
}

type RoomMessage struct {
	ID             string    `json:"id"`
	RoomID         string    `json:"room_id"`
	SenderID       string    `json:"sender_id"`
	SenderName     string    `json:"sender_name"`
	SenderUsername string    `json:"sender_username"`
	SenderAvatar   string    `json:"sender_avatar_url,omitempty"`
	Text           string    `json:"text"`
	CreatedAt      time.Time `json:"created_at"`
}

type CreateRoomRequest struct {
	// Type and IsPublic are overridden by the service; clients may omit them.
	Type        string `json:"type"`
	Name        string `json:"name"        validate:"required,min=1,max=120"`
	Description string `json:"description" validate:"omitempty,max=500"`
	CoverURL    string `json:"cover_url"   validate:"omitempty,max=500"`
	IsPublic    bool   `json:"is_public"`
}

type InviteMemberRequest struct {
	UserID string `json:"user_id" validate:"required"`
}

type UpdateRoomRequest struct {
	Name        string `json:"name"        validate:"required,min=1,max=120"`
	Description string `json:"description" validate:"omitempty,max=500"`
	CoverURL    string `json:"cover_url"   validate:"omitempty,max=500"`
}

// RoomMember is a full member entry returned by GET /rooms/:id/members.
type RoomMember struct {
	UserID    string    `json:"user_id"`
	FullName  string    `json:"full_name"`
	Username  string    `json:"username"`
	AvatarURL string    `json:"avatar_url,omitempty"`
	IsMuted   bool      `json:"is_muted"`
	IsCreator bool      `json:"is_creator"`
	IsAdmin   bool      `json:"is_admin"`
	JoinedAt  time.Time `json:"joined_at"`
}

var ErrRoomNotFound  = newRoomErr("room not found")
var ErrRoomClosed    = newRoomErr("room is closed")
var ErrNotInRoom     = newRoomErr("not a member of this room")
var ErrPrivateRoom   = newRoomErr("room is private — invite only")
var ErrNotInVoice    = newRoomErr("not in voice channel")

func newRoomErr(s string) error { return roomErrString(s) }

type roomErrString string

func (e roomErrString) Error() string { return string(e) }
