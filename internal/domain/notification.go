package domain

import "time"

const (
	NotificationTypeLike       = "like"
	NotificationTypeComment    = "comment"
	NotificationTypeFollow     = "follow"
	NotificationTypeMention    = "mention"
	NotificationTypeStoryLike  = "story_like"
	NotificationTypeMissedCall = "missed_call" // C-8
)

type Notification struct {
	ID          string     `json:"id" db:"id"`
	UserID      string     `json:"user_id" db:"user_id"`
	FromUserID  *string    `json:"from_user_id,omitempty" db:"from_user_id"`
	Type        string     `json:"type" db:"type"`
	EntityID    *string    `json:"entity_id,omitempty" db:"entity_id"`
	EntityType  *string    `json:"entity_type,omitempty" db:"entity_type"`
	// CommentID — для deep-link'а к конкретному комментарию (type=comment/reply/mention).
	// EntityID при этом по-прежнему = post_id (для роутинга на /post/:id).
	// Frontend использует commentId для scroll-to-comment после открытия поста.
	CommentID   *string    `json:"comment_id,omitempty" db:"comment_id"`
	Message     string     `json:"message" db:"message"`
	IsRead      bool       `json:"is_read" db:"is_read"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`

	// Joined fields
	FromUser *UserShort `json:"from_user,omitempty"`

	// Batching: когда несколько похожих нотификаций (один тип, тот же entity,
	// в пределах 1 часа) объединены в одну запись для фронта.
	// OthersCount — сколько ЕЩЁ юзеров присоединилось к этому действию помимо FromUser.
	// OtherUsers — превью аватарок (макс 3) других юзеров батча, для рендера группового списка.
	OthersCount int          `json:"others_count,omitempty"`
	OtherUsers  []*UserShort `json:"other_users,omitempty"`
}

type WSMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}
