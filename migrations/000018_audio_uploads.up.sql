-- User-uploaded tracks. Existing seeded tracks have user_id NULL and status='approved'.
ALTER TABLE audio_tracks ADD COLUMN IF NOT EXISTS user_id UUID REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE audio_tracks ADD COLUMN IF NOT EXISTS status VARCHAR(20) NOT NULL DEFAULT 'approved';
ALTER TABLE audio_tracks ADD COLUMN IF NOT EXISTS rejection_reason TEXT NOT NULL DEFAULT '';
ALTER TABLE audio_tracks ADD COLUMN IF NOT EXISTS reviewed_at TIMESTAMPTZ;
ALTER TABLE audio_tracks ADD COLUMN IF NOT EXISTS reviewed_by UUID REFERENCES users(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_audio_tracks_status ON audio_tracks(status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audio_tracks_user ON audio_tracks(user_id, created_at DESC) WHERE user_id IS NOT NULL;
