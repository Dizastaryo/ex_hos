package domain

import "time"

const (
	LikeEntityPost    = "post"
	LikeEntityComment = "comment"
	LikeEntityStory   = "story"
	LikeEntityScanner = "scanner" // лайк в BLE-сканере
	LikeEntityReel    = "reel"
	LikeEntityAudio   = "audio"
	LikeEntityVideo   = "video"
	LikeEntityFile    = "file" // книги и прочие файлы
)

type Like struct {
	ID         string    `json:"id" db:"id"`
	UserID     string    `json:"user_id" db:"user_id"`
	EntityID   string    `json:"entity_id" db:"entity_id"`
	EntityType string    `json:"entity_type" db:"entity_type"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
}
