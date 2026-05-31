-- MUSIC-2: LRC-формат lyrics для аудио-треков.
-- Опционально, NULL = без lyrics. Frontend парсит «[mm:ss.xx]line» и
-- отображает текущую строку synced с player.position.

ALTER TABLE audio_tracks
    ADD COLUMN IF NOT EXISTS lyrics_lrc TEXT;
