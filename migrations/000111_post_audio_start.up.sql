-- Audio start offset for posts with a music overlay (photo OR video).
-- Frontend _audioStartSec.round() is stored here; the viewer seeks the track
-- to this offset before playback. For a photo post the track then loops from
-- this point forever.

ALTER TABLE posts
    ADD COLUMN IF NOT EXISTS audio_start_seconds INT NOT NULL DEFAULT 0
        CHECK (audio_start_seconds >= 0);
