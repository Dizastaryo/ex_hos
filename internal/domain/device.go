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

// ScanProfile — анонимная личность юзера в BLE-сканере.
// Сканирующий видит ТОЛЬКО эти поля, реальный аккаунт скрыт.
type ScanProfile struct {
	ScanAlias     string `json:"scan_alias"`
	ScanAvatarURL string `json:"scan_avatar_url"`
	// DeviceHash — public_id_hex браслета. Используется для отправки лайка.
	DeviceHash    string `json:"device_hash"`
}

// UpdateScanProfileRequest — тело PUT /users/me/scan-profile.
type UpdateScanProfileRequest struct {
	ScanAlias     string `json:"scan_alias"      validate:"omitempty,max=50"`
	ScanAvatarURL string `json:"scan_avatar_url" validate:"omitempty,max=500"`
	ScanEnabled   *bool  `json:"scan_enabled"`
}

// GenerateDevicesRequest — тело POST /admin/devices/generate.
type GenerateDevicesRequest struct {
	Count int    `json:"count" validate:"required,min=1,max=500"`
	Notes string `json:"notes" validate:"omitempty,max=200"`
}
