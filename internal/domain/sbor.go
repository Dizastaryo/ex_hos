package domain

import "time"

type Sbor struct {
	ID           string     `json:"id"            db:"id"`
	HostID       string     `json:"host_id"       db:"host_id"`
	Type         string     `json:"type"          db:"type"`
	Category     string     `json:"category"      db:"category"`
	Title        string     `json:"title"         db:"title"`
	Place        string     `json:"place"         db:"place"`
	City         string     `json:"city"          db:"city"`
	Description  string     `json:"description"   db:"description"`
	ScheduledAt  *time.Time `json:"scheduled_at"  db:"scheduled_at"`
	FlexibleTime bool       `json:"flexible_time" db:"flexible_time"`
	MaxSlots     *int       `json:"max"           db:"max_slots"`
	IsLive       bool       `json:"live"          db:"is_live"`
	IsCancelled  bool       `json:"is_cancelled"  db:"is_cancelled"`
	CreatedAt    time.Time  `json:"created_at"    db:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"    db:"updated_at"`

	CoverUrl string `json:"cover_url" db:"cover_url"`
	Price    int    `json:"price"     db:"price"`

	// Group chat linked to this sbor (set on creation)
	ChatID *string `json:"chat_id" db:"chat_id"`

	// Joined/computed
	HostName        string   `json:"host_name"`
	Joined          int      `json:"joined"`
	MemberNames      []string `json:"member_names"`
	MemberUsernames  []string `json:"member_usernames"`
	MemberIDs        []string `json:"member_ids"`
	MemberAvatarURLs []string `json:"member_avatar_urls"`
	MyRole          string   `json:"my_role"`   // "" | "participant" | "organizer"
	IsJoined        bool     `json:"is_joined"`
	IsBookmarked    bool     `json:"is_bookmarked"`
	When            string   `json:"when"`
	WhenSub         string   `json:"when_sub,omitempty"`
	Distance        string   `json:"distance,omitempty"`

	// Request flow fields
	MyRequestStatus      string `json:"my_request_status"`       // "" | "pending" | "approved" | "rejected"
	PendingRequestsCount int    `json:"pending_requests_count"`  // только для organizer'а
}

// SborJoinRequest — заявка участника на вступление в сбор.
type SborJoinRequest struct {
	ID        string    `json:"id"`
	SborID    string    `json:"sbor_id"`
	UserID    string    `json:"user_id"`
	Username  string    `json:"username"`
	FullName  string    `json:"full_name"`
	AvatarURL string    `json:"avatar_url"`
	Status    string    `json:"status"` // pending | approved | rejected
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateSborRequest struct {
	Type         string     `json:"type"          validate:"required,oneof=offline online"`
	Category     string     `json:"category"      validate:"required,max=50"`
	Title        string     `json:"title"         validate:"required,min=3,max=120"`
	Place        string     `json:"place"         validate:"omitempty,max=255"`
	City         string     `json:"city"          validate:"omitempty,max=100"`
	CoverUrl     string     `json:"cover_url"     validate:"omitempty,max=500"`
	Price        int        `json:"price"         validate:"omitempty,min=0"`
	Description  string     `json:"description"   validate:"omitempty,max=1000"`
	ScheduledAt  *time.Time `json:"scheduled_at"`
	FlexibleTime bool       `json:"flexible_time"`
	MaxSlots     *int       `json:"max_slots"     validate:"omitempty,min=2,max=1000"`
}

type UpdateSborRequest struct {
	Title        *string    `json:"title"         validate:"omitempty,min=3,max=120"`
	Place        *string    `json:"place"         validate:"omitempty,max=255"`
	CoverUrl     *string    `json:"cover_url"     validate:"omitempty,max=500"`
	Description  *string    `json:"description"   validate:"omitempty,max=1000"`
	Price        *int       `json:"price"         validate:"omitempty,min=0"`
	Category     *string    `json:"category"      validate:"omitempty,max=50"`
	ScheduledAt  *time.Time `json:"scheduled_at"`
	FlexibleTime *bool      `json:"flexible_time"`
	MaxSlots     *int       `json:"max_slots"     validate:"omitempty,min=2,max=1000"`
}

var ErrSborNotFound      = newErr("sbor not found")
var ErrSborFull          = newErr("sbor is full")
var ErrAlreadyJoined     = newErr("already joined")
var ErrNotJoined         = newErr("not a member of this sbor")
var ErrRequestNotFound   = newErr("request not found")
var ErrAlreadyRequested  = newErr("request already pending")
var ErrMaxSlotsConflict  = newErr("max slots cannot be less than current member count")

func newErr(s string) error { return errString(s) }

type errString string

func (e errString) Error() string { return string(e) }
