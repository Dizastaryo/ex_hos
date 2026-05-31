-- Audio track для photo-stories: Spotify-style музыка поверх фото.
-- Для video-stories пока не используется (звук в самом видео).
-- FK на audio_tracks (миграция 6, video bundle). ON DELETE SET NULL —
-- если трек удалили, story остаётся видимой без музыки.
ALTER TABLE stories
    ADD COLUMN IF NOT EXISTS audio_track_id UUID
        REFERENCES audio_tracks(id) ON DELETE SET NULL;
