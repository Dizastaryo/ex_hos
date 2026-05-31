-- File likes counter. Сами лайки живут в polymorphic `likes` (entity_type='file').
-- likes_count денормализован — растёт/уменьшается в коде сервиса.
ALTER TABLE files
    ADD COLUMN IF NOT EXISTS likes_count INT NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_files_likes_count ON files(likes_count DESC);
