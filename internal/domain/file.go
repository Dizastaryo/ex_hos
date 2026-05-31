package domain

import (
	"errors"
	"time"
)

var (
	ErrFileNotFound = errors.New("file not found")
)

type FileCategory struct {
	ID        string    `json:"id" db:"id"`
	Name      string    `json:"name" db:"name"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

type File struct {
	ID             string    `json:"id" db:"id"`
	UserID         string    `json:"user_id" db:"user_id"`
	Filename       string    `json:"filename" db:"filename"`
	FileURL        string    `json:"file_url" db:"file_url"`
	MimeType       string    `json:"mime_type" db:"mime_type"`
	FileSize       int64     `json:"file_size" db:"file_size"`
	CategoryID     string    `json:"category_id" db:"category_id"`
	DownloadsCount int       `json:"downloads_count" db:"downloads_count"`
	LikesCount     int       `json:"likes_count" db:"likes_count"`
	IsPreviewable  bool      `json:"is_previewable" db:"is_previewable"`
	PreviewURL     string    `json:"preview_url" db:"preview_url"`
	Description    string    `json:"description" db:"description"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`

	User     *UserShort    `json:"user,omitempty"`
	Category *FileCategory `json:"category,omitempty"`
	// IsLiked для текущего viewer — заполняется handler'ом, не сканится из БД.
	IsLiked bool `json:"is_liked,omitempty"`
}

type CreateFileRequest struct {
	Filename    string `json:"filename" validate:"required,min=1,max=500"`
	FileURL     string `json:"file_url" validate:"required"`
	MimeType    string `json:"mime_type" validate:"required"`
	FileSize    int64  `json:"file_size" validate:"required,min=1"`
	CategoryID  string `json:"category_id" validate:"omitempty"`
	Description string `json:"description" validate:"omitempty,max=1000"`
}
