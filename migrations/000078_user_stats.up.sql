-- Social score: агрегированные лайки по всем типам контента.
-- Обновляется при каждом лайке (atomic increment), читается при просмотре профиля.
-- Отдельные счётчики по типам нужны для аналитики и визуализации breakdown'а.
CREATE TABLE user_stats (
    user_id       UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    total_likes   INT NOT NULL DEFAULT 0,
    scanner_likes INT NOT NULL DEFAULT 0,
    post_likes    INT NOT NULL DEFAULT 0,
    story_likes   INT NOT NULL DEFAULT 0,
    reel_likes    INT NOT NULL DEFAULT 0,
    audio_likes   INT NOT NULL DEFAULT 0,
    video_likes   INT NOT NULL DEFAULT 0,
    book_likes    INT NOT NULL DEFAULT 0,
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Инициализировать строку для всех существующих пользователей (все нули).
INSERT INTO user_stats (user_id)
SELECT id FROM users
ON CONFLICT DO NOTHING;
