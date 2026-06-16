-- Scanner v2: "Waves" (proximity-based greetings)
-- Replaces the old "likes" concept with proximity-restricted waves.

-- Add scan_emoji and scan_status to users for anonymous scan-profile
ALTER TABLE users ADD COLUMN IF NOT EXISTS scan_emoji TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS scan_status TEXT NOT NULL DEFAULT '';

-- Waves table: records when user A waves at user B (identified by device_hash)
-- Reuses the same data as scanner_likes but adds per-target cooldown.
-- We keep scanner_likes for backwards compat; new code uses scanner_waves.
CREATE TABLE IF NOT EXISTS scanner_waves (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    waver_id        UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    target_user_id  UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Target reads incoming waves (sorted by time)
CREATE INDEX IF NOT EXISTS idx_scanner_waves_target ON scanner_waves(target_user_id, created_at DESC);
-- Waver checks cooldown per target
CREATE INDEX IF NOT EXISTS idx_scanner_waves_cooldown ON scanner_waves(waver_id, target_user_id, created_at DESC);
-- Daily count for rate limit
CREATE INDEX IF NOT EXISTS idx_scanner_waves_daily ON scanner_waves(waver_id, created_at);
