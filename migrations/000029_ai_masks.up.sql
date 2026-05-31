-- AI-сгенерированные маски юзеров. file_url — локальный путь
-- к скачанному PNG в /uploads/ai/masks/. DALL-E URLs expir'ятся
-- через час; мы скачиваем результат сразу и храним сами.

CREATE TABLE IF NOT EXISTS ai_masks (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    prompt TEXT NOT NULL,
    file_url TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ai_masks_user_created
    ON ai_masks(user_id, created_at DESC);
