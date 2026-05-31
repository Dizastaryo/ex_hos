package domain

import "time"

type Comment struct {
	ID         string    `json:"id" db:"id"`
	PostID     string    `json:"post_id" db:"post_id"`
	UserID     string    `json:"user_id" db:"user_id"`
	ParentID   *string   `json:"parent_id,omitempty" db:"parent_id"`
	Text       string    `json:"text" db:"text"`
	LikesCount int       `json:"likes_count" db:"likes_count"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`

	// Joined fields
	User      *UserShort `json:"user,omitempty"`
	IsLiked   bool       `json:"is_liked,omitempty"`
	RepliesCount int     `json:"replies_count,omitempty"`
}

type CreateCommentRequest struct {
	Text     string  `json:"text" validate:"required,min=1,max=2200"`
	ParentID *string `json:"parent_id" validate:"omitempty,uuid"`
}
