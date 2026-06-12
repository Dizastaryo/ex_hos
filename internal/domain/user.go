package domain

import "time"

type User struct {
	ID              string    `json:"id" db:"id"`
	Username        string    `json:"username" db:"username"`
	Phone           string    `json:"phone,omitempty" db:"phone"`
	FullName        string    `json:"full_name" db:"full_name"`
	Bio             string    `json:"bio" db:"bio"`
	AvatarURL       string    `json:"avatar_url" db:"avatar_url"`
	Website         string    `json:"website" db:"website"`
	Gender          string    `json:"gender" db:"gender"`
	DateOfBirth     *time.Time `json:"date_of_birth,omitempty" db:"date_of_birth"`
	DevicePublicID  string    `json:"device_public_id" db:"device_public_id"`
	DevicePrivateID string    `json:"device_private_id,omitempty" db:"device_private_id"`
	IsPrivate       bool      `json:"is_private" db:"is_private"`
	IsVerified      bool      `json:"is_verified" db:"is_verified"`
	IsAdmin         bool      `json:"is_admin,omitempty" db:"is_admin"`
	IsBanned        bool      `json:"is_banned,omitempty" db:"is_banned"`
	BannedReason    string    `json:"banned_reason,omitempty" db:"banned_reason"`
	PostsCount      int       `json:"posts_count" db:"posts_count"`
	FollowersCount  int       `json:"followers_count" db:"followers_count"`
	FollowingCount  int       `json:"following_count" db:"following_count"`
	LastSeenAt      time.Time `json:"last_seen_at" db:"last_seen_at"`
	// HideLastSeen — PROFILE-6 privacy toggle. true → бэк скрывает is_online
	// и last_seen_at от других зрителей (для self — всегда виден).
	HideLastSeen    bool      `json:"hide_last_seen" db:"hide_last_seen"`
	// VIDEO-4 channel fields: ChannelAbout = «О канале» текст, ChannelBannerURL
	// = hero-баннер 16:9 поверх профиля. Если оба пусты — профиль обычный.
	ChannelAbout     string `json:"channel_about" db:"channel_about"`
	ChannelBannerURL string `json:"channel_banner_url" db:"channel_banner_url"`
	// Scan-профиль: анонимная личность в BLE-сканере.
	// Сканирующий видит scan_alias + scan_avatar_url, реальный аккаунт скрыт.
	// scan_enabled=false → /by-device отдаёт 404 (сервер-сайд privacy toggle).
	ScanAlias     string `json:"scan_alias" db:"scan_alias"`
	ScanAvatarURL string `json:"scan_avatar_url" db:"scan_avatar_url"`
	ScanEnabled   bool   `json:"scan_enabled" db:"scan_enabled"`
	CreatedAt       time.Time `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time `json:"updated_at" db:"updated_at"`

	// Computed fields (not in DB)
	IsFollowing  bool `json:"is_following,omitempty"`
	IsFollowedBy bool `json:"is_followed_by,omitempty"`
}

// SendOTPRequest - request to send OTP to phone
type SendOTPRequest struct {
	Phone string `json:"phone" validate:"required,min=10,max=20"`
}

// VerifyOTPRequest - verify OTP code and login/register.
// AcceptsTerms is required by the front-end consent UI; the server records the
// timestamp on a new registration and refreshes it on every successful login,
// so we always know when the current owner of the account last agreed to terms.
type VerifyOTPRequest struct {
	Phone        string `json:"phone" validate:"required,min=10,max=20"`
	Code         string `json:"code" validate:"required,len=4"`
	AcceptsTerms bool   `json:"accepts_terms"`
	// InviteCode, if present and the user is brand-new, attributes the
	// signup to the inviter. Optional, ignored for existing users.
	InviteCode string `json:"invite_code,omitempty" validate:"omitempty,max=20"`
}

type UpdateProfileRequest struct {
	FullName        string `json:"full_name" validate:"omitempty,min=1,max=100"`
	Username        string `json:"username" validate:"omitempty,min=3,max=30,alphanum"`
	Bio             string `json:"bio" validate:"omitempty,max=500"`
	Website         string `json:"website" validate:"omitempty,max=255"`
	Gender          string `json:"gender" validate:"omitempty,max=10"`
	IsPrivate       *bool  `json:"is_private"`
	// HideLastSeen — PROFILE-6 privacy toggle. *bool чтобы omit означал «не менять».
	HideLastSeen    *bool  `json:"hide_last_seen"`
	AvatarURL       string `json:"avatar_url" validate:"omitempty,max=500"`
	DevicePublicID  string `json:"device_public_id" validate:"omitempty,max=255"`
	DevicePrivateID string `json:"device_private_id" validate:"omitempty,max=255"`
	// VIDEO-4 channel fields. Pointers — отсутствие в payload = не менять.
	ChannelAbout     *string `json:"channel_about" validate:"omitempty"`
	ChannelBannerURL *string `json:"channel_banner_url" validate:"omitempty,max=500"`
}

type UserPublicProfile struct {
	ID              string    `json:"id"`
	Username        string    `json:"username"`
	FullName        string    `json:"full_name"`
	Bio             string    `json:"bio"`
	AvatarURL       string    `json:"avatar_url"`
	Website         string    `json:"website"`
	Gender          string    `json:"gender"`
	DevicePublicID  string    `json:"device_public_id"`
	IsPrivate       bool      `json:"is_private"`
	IsVerified      bool      `json:"is_verified"`
	PostsCount      int       `json:"posts_count"`
	FollowersCount  int       `json:"followers_count"`
	FollowingCount  int       `json:"following_count"`
	IsFollowing     bool      `json:"is_following"`
	IsFollowedBy    bool      `json:"is_followed_by"`
	// HasPendingFollowRequest = viewer запросил подписку на этого приватного юзера, но запрос ещё не подтверждён.
	// Используется чтобы Follow-кнопка показывала «Запрос отправлен» вместо «Подписаться».
	HasPendingFollowRequest bool `json:"has_pending_follow_request"`
	// Presence: IsOnline = есть хотя бы одно WS-соединение прямо сейчас.
	// LastSeenAt = последнее обновление (connect или disconnect). Фронт
	// показывает «в сети» или «был N мин назад».
	IsOnline   bool      `json:"is_online"`
	LastSeenAt time.Time `json:"last_seen_at"`
	// VIDEO-4 channel fields. Если оба пусты — UI рендерит обычный профиль;
	// иначе — channel-mode с banner + about поверх profile-header.
	ChannelAbout     string `json:"channel_about"`
	ChannelBannerURL string `json:"channel_banner_url"`
	// TotalLikes — суммарный социальный счёт из user_stats.
	TotalLikes int `json:"total_likes"`
	CreatedAt  time.Time `json:"created_at"`
}

type UserShort struct {
	ID         string `json:"id"`
	Username   string `json:"username"`
	FullName   string `json:"full_name"`
	AvatarURL  string `json:"avatar_url"`
	IsVerified bool   `json:"is_verified"`
}
