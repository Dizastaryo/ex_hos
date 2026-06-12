package domain

import "time"

type AudioTrack struct {
	ID              string    `json:"id" db:"id"`
	Title           string    `json:"title" db:"title"`
	Artist          string    `json:"artist" db:"artist"`
	CoverURL        string    `json:"cover_url" db:"cover_url"`
	AudioURL        string    `json:"audio_url" db:"audio_url"`
	DurationSeconds int       `json:"duration_seconds" db:"duration_seconds"`
	UsesCount       int       `json:"uses_count" db:"uses_count"`
	Genre           string    `json:"genre" db:"genre"`
	CreatedAt       time.Time `json:"created_at" db:"created_at"`

	// Upload moderation. Empty UserID + status='approved' = seeded official catalog.
	UserID           string `json:"user_id,omitempty" db:"user_id"`
	Status           string `json:"status" db:"status"` // pending | approved | rejected
	RejectionReason  string `json:"rejection_reason,omitempty" db:"rejection_reason"`
	// MUSIC-2: LRC-формат lyrics («[mm:ss.xx]Line»). nil/empty = без lyrics.
	// Frontend парсит и рендерит sing-along scroller.
	LyricsLRC  string `json:"lyrics_lrc,omitempty" db:"lyrics_lrc"`
	LikesCount int    `json:"likes_count" db:"likes_count"`
	// IsLiked — заполняется handler'ом для авторизованного viewer'а.
	IsLiked bool `json:"is_liked,omitempty"`
}

type TrendingTag struct {
	Tag        string `json:"tag"`
	PostsCount int    `json:"posts_count"`
}
