DROP INDEX IF EXISTS idx_audio_tracks_user;
DROP INDEX IF EXISTS idx_audio_tracks_status;
ALTER TABLE audio_tracks DROP COLUMN IF EXISTS reviewed_by;
ALTER TABLE audio_tracks DROP COLUMN IF EXISTS reviewed_at;
ALTER TABLE audio_tracks DROP COLUMN IF EXISTS rejection_reason;
ALTER TABLE audio_tracks DROP COLUMN IF EXISTS status;
ALTER TABLE audio_tracks DROP COLUMN IF EXISTS user_id;
