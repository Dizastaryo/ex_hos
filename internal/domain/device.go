package domain

import "time"

// BleDevice — одна запись в таблице ble_devices.
// Каждая запись = один физический браслет SeeU Band.
// public_id_hex / private_id_hex генерируются через HMAC(masterSecret, serial+"_pub/_prv")
// и вшиваются в прошивку браслета. Юзер привязывает браслет по serial_number с QR-наклейки.
type BleDevice struct {
	ID            string    `json:"id"             db:"id"`
	SerialNumber  string    `json:"serial_number"  db:"serial_number"`
	PublicIDHex   string    `json:"public_id_hex"  db:"public_id_hex"`
	// PrivateIDHex отдаётся только в CSV-экспорте для прошивки, не в обычном API
	PrivateIDHex  string    `json:"-"              db:"private_id_hex"`
	IsActive      bool      `json:"is_active"      db:"is_active"`
	Notes         string    `json:"notes"          db:"notes"`
	CreatedAt     time.Time `json:"created_at"     db:"created_at"`
}

// BleDeviceExportRow — строка CSV для скрипта прошивки (содержит оба хэша).
type BleDeviceExportRow struct {
	SerialNumber  string `json:"serial_number"`
	PublicIDHex   string `json:"public_id_hex"`
	PrivateIDHex  string `json:"private_id_hex"`
}

// ScanProfile — REAL profile shown in BLE scanner.
// Scanner shows actual account info to nearby users.
type ScanProfile struct {
	UserID     string `json:"user_id"`
	Username   string `json:"username"`
	FullName   string `json:"full_name"`
	AvatarURL  string `json:"avatar_url"`
	Bio        string `json:"bio"`
	IsVerified bool   `json:"is_verified"`
	// DeviceHash — public_id_hex браслета. Используется для отправки лайка.
	DeviceHash string `json:"device_hash"`
}

// UpdateScanProfileRequest — тело PUT /users/me/scan-profile.
type UpdateScanProfileRequest struct {
	ScanEnabled *bool `json:"scan_enabled"`
}

// ConnectToken — short-lived QR token for starting a chat via physical contact.
type ConnectToken struct {
	Token     string    `json:"token"`
	QRValue   string    `json:"qr_value"`
	ExpiresAt time.Time `json:"expires_at"`
}

// GenerateDevicesRequest — тело POST /admin/devices/generate.
type GenerateDevicesRequest struct {
	Count int    `json:"count" validate:"required,min=1,max=500"`
	Notes string `json:"notes" validate:"omitempty,max=200"`
}

// PrivateWhitelistEntry — один разрешённый пользователь в private-whitelist.
type PrivateWhitelistEntry struct {
	UserID    string `json:"user_id"`
	Username  string `json:"username"`
	FullName  string `json:"full_name"`
	AvatarURL string `json:"avatar_url"`
}

// SetPrivateWhitelistRequest — тело PUT /users/me/private-whitelist.
// UserIDs — список user_id взаимных подписчиков которым разрешён private-режим.
// Пустой список [] = никто не видит (максимальная приватность).
type SetPrivateWhitelistRequest struct {
	UserIDs []string `json:"user_ids" validate:"max=500"`
}
