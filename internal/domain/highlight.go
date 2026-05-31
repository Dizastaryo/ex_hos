package domain

import "time"

type Highlight struct {
	ID        string    `json:"id" db:"id"`
	UserID    string    `json:"user_id" db:"user_id"`
	Title     string    `json:"title" db:"title"`
	CoverURL  string    `json:"cover_url" db:"cover_url"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`

	// Joined fields
	Stories []*Story `json:"stories,omitempty"`
}

type CreateHighlightRequest struct {
	Title    string   `json:"title" validate:"required,min=1,max=50"`
	CoverURL string   `json:"cover_url" validate:"omitempty,url"`
	StoryIDs []string `json:"story_ids" validate:"required,min=1,dive,uuid"`
}

type UpdateHighlightRequest struct {
	Title    string   `json:"title" validate:"omitempty,min=1,max=50"`
	CoverURL string   `json:"cover_url" validate:"omitempty,url"`
	StoryIDs []string `json:"story_ids" validate:"omitempty,min=1,dive,uuid"`
}
