package domain

import (
	"encoding/json"
	"time"
)

const (
	ReadingStatusWant    = "want"
	ReadingStatusReading = "reading"
	ReadingStatusDone    = "done"
)

type ReadingProgress struct {
	UserID     string          `json:"user_id"`
	FileID     string          `json:"file_id"`
	Position   json.RawMessage `json:"position"`
	LastReadAt time.Time       `json:"last_read_at"`
}

type FileBookmark struct {
	ID        string          `json:"id"`
	UserID    string          `json:"user_id"`
	FileID    string          `json:"file_id"`
	Position  json.RawMessage `json:"position"`
	Note      string          `json:"note"`
	CreatedAt time.Time       `json:"created_at"`
}

type ReadingStatus struct {
	UserID    string    `json:"user_id"`
	FileID    string    `json:"file_id"`
	Status    string    `json:"status"`
	UpdatedAt time.Time `json:"updated_at"`
}

type UpsertProgressRequest struct {
	Position json.RawMessage `json:"position" validate:"required"`
}

type CreateBookmarkRequest struct {
	Position json.RawMessage `json:"position"`
	Note     string          `json:"note" validate:"omitempty,max=1000"`
}

type UpsertReadingStatusRequest struct {
	Status string `json:"status" validate:"required,oneof=want reading done"`
}

type ReadingGoal struct {
	UserID    string    `json:"user_id"`
	Year      int       `json:"year"`
	GoalBooks int       `json:"goal_books"`
	DoneBooks int       `json:"done_books"` // computed: files with status='done' this year
	UpdatedAt time.Time `json:"updated_at"`
}

type UpsertReadingGoalRequest struct {
	GoalBooks int `json:"goal_books" validate:"required,min=1,max=1000"`
}
