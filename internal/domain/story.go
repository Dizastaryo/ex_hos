package domain

import "time"

type Story struct {
	ID          string    `json:"id" db:"id"`
	UserID      string    `json:"user_id" db:"user_id"`
	MediaURL    string    `json:"media_url" db:"media_url"`
	MediaType   string    `json:"media_type" db:"media_type"`
	Duration    int       `json:"duration" db:"duration"`
	TextOverlay string    `json:"text_overlay" db:"text_overlay"`
	ViewsCount  int       `json:"views_count" db:"views_count"`
	LikesCount  int       `json:"likes_count" db:"likes_count"`
	// AudioTrackID — Spotify-style музыка поверх photo-story.
	// nil = без музыки. Frontend плеер мики'кует трек когда story активен.
	AudioTrackID *string   `json:"audio_track_id,omitempty" db:"audio_track_id"`
	// AudioStartSeconds — MUSIC-7. С какой секунды трека начать playback в
	// viewer'е (по умолчанию 0). Frontend slider в media_prepare_screen.
	AudioStartSeconds int `json:"audio_start_seconds" db:"audio_start_seconds"`
	// SharedPostID — если сторис создан через share-post-to-stories (CHAT-5),
	// сюда пишется id оригинального поста. Frontend viewer рисует badge
	// «От @author» tappable → /post/:id. nil = обычная сторис.
	SharedPostID *string   `json:"shared_post_id,omitempty" db:"shared_post_id"`
	// BgColor — STORY-1. Используется когда MediaType == 'text': hex (#RRGGBB)
	// или название preset-градиента (sunset / ocean / forest / mono). Frontend
	// рендерит Container с этим background'ом и центральным текстом из
	// TextOverlay. Для image/video — пустая строка.
	BgColor     string    `json:"bg_color" db:"bg_color"`
	// Poll — STORY-3. Интерактивный poll-overlay. nil = poll'а нет.
	// Хранится в БД как JSONB {question, option_a, option_b}. Voters в
	// отдельной таблице story_poll_votes; счётчики/мой_голос hydrate'ятся
	// per-viewer в Hydrate'ах.
	Poll *StoryPoll `json:"poll,omitempty" db:"poll"`
	// IsCloseFriendsOnly — PROFILE-3. true = story виден только close_friends
	// автора + ему самому. Story-feed/GetByUsername фильтруют автоматически.
	// Frontend: stories-row рисует зелёный ring вокруг preview.
	IsCloseFriendsOnly bool `json:"is_close_friends_only" db:"is_close_friends_only"`
	ExpiresAt   time.Time `json:"expires_at" db:"expires_at"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`

	// Computed fields
	User    *UserShort `json:"user,omitempty"`
	IsViewed bool      `json:"is_viewed,omitempty"`
	IsLiked  bool      `json:"is_liked,omitempty"`

	// Aggregate emoji-reaction counts and current viewer's reaction. Hydrated
	// per-viewer; empty map / empty string when no reactions or anon viewer.
	Reactions  map[string]int `json:"reactions,omitempty"`
	MyReaction string         `json:"my_reaction,omitempty"`
}

// StoryPoll — интерактивный 2-option poll поверх сторис. Question + два варианта.
// VotesA/VotesB/MyVote заполняются при чтении (не сохраняются в JSONB).
type StoryPoll struct {
	Question string `json:"question"`
	OptionA  string `json:"option_a"`
	OptionB  string `json:"option_b"`
	// Position 0..1 на canvas-9:16 — позиция overlay'я.
	X float64 `json:"x"`
	Y float64 `json:"y"`
	// Counts populate'ятся при выдаче.
	VotesA int `json:"votes_a,omitempty"`
	VotesB int `json:"votes_b,omitempty"`
	// MyVote: -1 = не голосовал, 0 или 1 = выбранный option.
	MyVote int `json:"my_vote,omitempty"`
}

type CreateStoryRequest struct {
	// MediaURL обязателен только для image/video — для text может быть пустым
	// (валидация ниже в service'е). Validate здесь без required чтобы text
	// проходил, image/video проверяется в service.CreateStory.
	MediaURL     string  `json:"media_url" validate:"omitempty"`
	MediaType    string  `json:"media_type" validate:"required,oneof=image video text"`
	TextOverlay  string  `json:"text_overlay" validate:"omitempty,max=500"`
	Duration     int     `json:"duration" validate:"omitempty,min=1,max=60"`
	AudioTrackID *string `json:"audio_track_id" validate:"omitempty,uuid"`
	SharedPostID *string `json:"shared_post_id" validate:"omitempty,uuid"`
	// MUSIC-7: offset playback'а от начала трека. 0 = с начала.
	AudioStartSeconds int `json:"audio_start_seconds" validate:"omitempty,min=0"`
	// STORY-1: фон для text-сторис. Hex (#RRGGBB) или preset-имя градиента.
	BgColor string `json:"bg_color" validate:"omitempty,max=40"`
	// STORY-3: интерактивный poll. nil = без poll'я.
	Poll *StoryPoll `json:"poll" validate:"omitempty"`
	// PROFILE-3: опубликовать только для close_friends. Default false.
	IsCloseFriendsOnly bool `json:"is_close_friends_only"`
}

type StoryFeedGroup struct {
	User    *UserShort `json:"user"`
	Stories []*Story   `json:"stories"`
	HasSeen bool       `json:"has_seen"`
}

type StoryViewer struct {
	User     *UserShort `json:"user"`
	ViewedAt time.Time  `json:"viewed_at"`
}
