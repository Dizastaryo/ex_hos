ALTER TABLE messages
    DROP COLUMN IF EXISTS media_duration_seconds,
    DROP COLUMN IF EXISTS waveform;
