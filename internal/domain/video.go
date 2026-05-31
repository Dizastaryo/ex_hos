package domain

import (
	"errors"
	"time"
)

var (
	ErrVideoNotFound = errors.New("video not found")
)

type VideoCategory struct {
	ID        string    `json:"id" db:"id"`
	Name      string    `json:"name" db:"name"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

type Video struct {
	ID              string    `json:"id" db:"id"`
	UserID          string    `json:"user_id" db:"user_id"`
	Title           string    `json:"title" db:"title"`
	Description     string    `json:"description" db:"description"`
	VideoURL        string    `json:"video_url" db:"video_url"`
	ThumbnailURL    string    `json:"thumbnail_url" db:"thumbnail_url"`
	DurationSeconds int       `json:"duration_seconds" db:"duration_seconds"`
	CategoryID      string    `json:"category_id" db:"category_id"`
	Resolution      string    `json:"resolution" db:"resolution"`
	ViewsCount      int       `json:"views_count" db:"views_count"`
	LikesCount      int       `json:"likes_count" db:"likes_count"`
	CommentsCount   int       `json:"comments_count" db:"comments_count"`
	IsLive          bool      `json:"is_live" db:"is_live"`
	// SubtitlesURL — VIDEO-5. VTT-формат URL (or empty). Frontend chewie
	// подгружает + рендерит overlay, ставит toggle-кнопку в controls.
	SubtitlesURL    string    `json:"subtitles_url" db:"subtitles_url"`
	CreatedAt       time.Time `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time `json:"updated_at" db:"updated_at"`

	User     *UserShort     `json:"user,omitempty"`
	Category *VideoCategory `json:"category,omitempty"`
	IsLiked  bool           `json:"is_liked,omitempty"`
}

type CreateVideoRequest struct {
	Title           string `json:"title" validate:"required,min=1,max=255"`
	Description     string `json:"description" validate:"omitempty,max=2000"`
	VideoURL        string `json:"video_url" validate:"required"`
	ThumbnailURL    string `json:"thumbnail_url" validate:"omitempty"`
	DurationSeconds int    `json:"duration_seconds" validate:"required,min=1"`
	CategoryID      string `json:"category_id" validate:"omitempty"`
	Resolution      string `json:"resolution" validate:"omitempty,max=10"`
	// VIDEO-5: VTT-subtitles URL. Опционально.
	SubtitlesURL    string `json:"subtitles_url" validate:"omitempty"`
}

