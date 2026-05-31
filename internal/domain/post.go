package domain

import "time"

type Post struct {
	ID            string    `json:"id" db:"id"`
	UserID        string    `json:"user_id" db:"user_id"`
	Caption       string    `json:"caption" db:"caption"`
	MediaURLs     []string  `json:"media_urls" db:"media_urls"`
	MediaTypes    []string  `json:"media_types" db:"media_types"`
	Location      string    `json:"location" db:"location"`
	ThumbnailURL  string    `json:"thumbnail_url" db:"thumbnail_url"`
	LikesCount    int       `json:"likes_count" db:"likes_count"`
	CommentsCount int       `json:"comments_count" db:"comments_count"`
	SavesCount    int       `json:"saves_count" db:"saves_count"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" db:"updated_at"`

	// Optional audio track overlay (used when post has video media —
	// previously a "reels" feature, now part of the unified post model).
	AudioTrackID string `json:"audio_track_id,omitempty" db:"audio_track_id"`

	// Joined fields
	User      *UserShort `json:"user,omitempty"`
	IsLiked   bool       `json:"is_liked,omitempty"`
	IsSaved   bool       `json:"is_saved,omitempty"`

	// Emoji reactions: count per emoji, plus the emoji *current user*
	// placed on this post (empty when none). Aggregated by repo so client
	// doesn't roundtrip per post.
	Reactions  map[string]int `json:"reactions,omitempty"`
	MyReaction string         `json:"my_reaction,omitempty"`
}

type CreatePostRequest struct {
	Caption      string   `json:"caption" validate:"omitempty,max=2200"`
	MediaURLs    []string `json:"media_urls" validate:"required,min=1,max=10"`
	MediaTypes   []string `json:"media_types" validate:"required,min=1,max=10,dive,oneof=image video"`
	Location     string   `json:"location" validate:"omitempty,max=255"`
	ThumbnailURL string   `json:"thumbnail_url" validate:"omitempty"`
	AudioTrackID string   `json:"audio_track_id" validate:"omitempty,uuid"`
}

type PostFeedItem struct {
	Post
	User    *UserShort `json:"user"`
	IsLiked bool       `json:"is_liked"`
	IsSaved bool       `json:"is_saved"`
}
