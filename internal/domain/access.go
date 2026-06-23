package domain

import "time"

// UserAccess — взаимный доступ между двумя пользователями.
// user_a_id < user_b_id (нормализовано: меньший UUID первым).
type UserAccess struct {
	ID        string    `json:"id"`
	UserAID   string    `json:"user_a_id"`
	UserBID   string    `json:"user_b_id"`
	GrantedAt time.Time `json:"granted_at"`
}

// AccessPartner — партнёр по доступу (для списка /access/list).
type AccessPartner struct {
	UserID     string    `json:"user_id"`
	Username   string    `json:"username"`
	FullName   string    `json:"full_name"`
	AvatarURL  string    `json:"avatar_url"`
	IsVerified bool      `json:"is_verified"`
	GrantedAt  time.Time `json:"granted_at"`
}

// ScanQRRequest — тело POST /access/scan.
type ScanQRRequest struct {
	Token string `json:"token" validate:"required"`
}
