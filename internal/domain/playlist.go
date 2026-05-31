package domain

import "time"

type Playlist struct {
	ID          string    `json:"id" db:"id"`
	UserID      string    `json:"user_id" db:"user_id"`
	Name        string    `json:"name" db:"name"`
	CoverURL    string    `json:"cover_url" db:"cover_url"`
	TracksCount int       `json:"tracks_count" db:"tracks_count"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

type PlaylistDetail struct {
	Playlist
	Tracks []*AudioTrack `json:"tracks"`
}
