package domain

import (
	"errors"
	"time"
)

var (
	ErrStreamNotFound   = errors.New("stream not found")
	ErrStreamEnded      = errors.New("stream already ended")
	ErrAlreadyStreaming = errors.New("user already has an active stream")
)

type LiveStream struct {
	ID          string     `json:"id"`
	UserID      string     `json:"user_id"`
	Username    string     `json:"username"`
	FullName    string     `json:"full_name"`
	AvatarURL   string     `json:"avatar_url"`
	Title       string     `json:"title"`
	Status      string     `json:"status"` // "live" | "ended"
	ViewerCount int        `json:"viewer_count"`
	StartedAt   time.Time  `json:"started_at"`
	EndedAt     *time.Time `json:"ended_at,omitempty"`
}

type LiveStreamViewer struct {
	UserID    string `json:"user_id"`
	Username  string `json:"username"`
	FullName  string `json:"full_name"`
	AvatarURL string `json:"avatar_url"`
}
