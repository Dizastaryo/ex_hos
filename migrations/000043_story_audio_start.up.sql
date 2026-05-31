-- MUSIC-7: audio start offset для stories с music overlay.
-- Frontend slider _audioStartSec.toInt() кладётся сюда; viewer на playback
-- seek(Duration(seconds: audio_start_seconds)) перед setUrl.

ALTER TABLE stories
    ADD COLUMN IF NOT EXISTS audio_start_seconds INT NOT NULL DEFAULT 0
        CHECK (audio_start_seconds >= 0);
