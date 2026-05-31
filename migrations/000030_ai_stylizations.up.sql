-- AI-стилизованные фото юзеров. source_url — оригинал (в /uploads/...),
-- result_url — стилизованный (в /uploads/ai/stylized/<uuid>.png).
-- style — preset id ('ghibli', 'pixar', ...) или 'custom' для свободного prompt'а.

CREATE TABLE IF NOT EXISTS ai_stylizations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    source_url TEXT NOT NULL,
    result_url TEXT NOT NULL,
    style VARCHAR(64) NOT NULL DEFAULT 'custom',
    prompt TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ai_stylizations_user_created
    ON ai_stylizations(user_id, created_at DESC);
