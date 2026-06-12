-- Лайки для аудио-треков (счётчик + поддержка social score).
-- Факт лайка хранится в polymorphic `likes` (entity_type='audio_track') — это уже есть.
-- Добавляем только денормализованный счётчик для быстрого чтения.
ALTER TABLE audio_tracks ADD COLUMN IF NOT EXISTS likes_count INT NOT NULL DEFAULT 0;

-- Backfill из существующих likes
UPDATE audio_tracks t
SET likes_count = (
    SELECT COUNT(*) FROM likes l
    WHERE l.entity_id = t.id AND l.entity_type = 'audio_track'
);
