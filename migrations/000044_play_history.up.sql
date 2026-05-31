-- MUSIC-3 + MUSIC-4: трекинг прослушиваний для smart-playlists и daily-mix.
--
-- Frontend audio_player_service шлёт POST /audio-tracks/:id/play на старт
-- (после ~5 сек чтобы случайные тапы не записывались). Backend пишет
-- (user_id, track_id, played_at) в play_history.
--
-- Smart-playlists:
--   liked  — JOIN c likes WHERE liker_id = $1 AND entity_type='track'
--   recent — DISTINCT track_id FROM play_history ORDER BY played_at DESC LIMIT N
-- Daily-mix:
--   Топ artists/genres из play_history → recent tracks из тех же категорий.

CREATE TABLE IF NOT EXISTS play_history (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    track_id    UUID NOT NULL REFERENCES audio_tracks(id) ON DELETE CASCADE,
    played_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    duration_played_sec INT NOT NULL DEFAULT 0
);

-- Для query «recent plays of user» + «top tracks/artists last week».
CREATE INDEX IF NOT EXISTS idx_play_history_user_time
    ON play_history(user_id, played_at DESC);
CREATE INDEX IF NOT EXISTS idx_play_history_track_time
    ON play_history(track_id, played_at DESC);
