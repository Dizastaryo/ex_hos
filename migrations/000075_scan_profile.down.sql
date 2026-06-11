DROP INDEX IF EXISTS idx_users_device_pub_scan;
ALTER TABLE users
    DROP COLUMN IF EXISTS scan_alias,
    DROP COLUMN IF EXISTS scan_avatar_url,
    DROP COLUMN IF EXISTS scan_enabled;
