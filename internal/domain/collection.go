package domain

import "time"

type Collection struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CoverFileID string    `json:"cover_file_id,omitempty"`
	FilesCount  int       `json:"files_count"`
	// CoverURLs — до 4 обложек файлов коллекции (для 2×2 коллажа).
	CoverURLs   []string  `json:"cover_urls,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// Populated for GetCollectionByID
	Files []*File `json:"files,omitempty"`
}

type CreateCollectionRequest struct {
	Name        string `json:"name" validate:"required,min=1,max=200"`
	Description string `json:"description" validate:"omitempty,max=1000"`
	CoverFileID string `json:"cover_file_id" validate:"omitempty"`
}

type UpdateCollectionRequest struct {
	Name        string `json:"name" validate:"required,min=1,max=200"`
	Description string `json:"description" validate:"omitempty,max=1000"`
	CoverFileID string `json:"cover_file_id" validate:"omitempty"`
}

type AddCollectionFileRequest struct {
	FileID string `json:"file_id" validate:"required"`
}
