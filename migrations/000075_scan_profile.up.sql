-- Scan-профиль: анонимная личность пользователя в BLE-сканере
-- Сканирующий видит scan_alias + scan_avatar_url, но не реальный username/avatar
-- scan_enabled=false → сервер отдаёт 404 на /by-device (даже если браслет рядом)

ALTER TABLE users
    ADD COLUMN scan_alias      TEXT    NOT NULL DEFAULT '',
    ADD COLUMN scan_avatar_url TEXT    NOT NULL DEFAULT '',
    ADD COLUMN scan_enabled    BOOLEAN NOT NULL DEFAULT TRUE;

CREATE INDEX idx_users_device_pub_scan ON users(device_public_id) WHERE scan_enabled = TRUE;
