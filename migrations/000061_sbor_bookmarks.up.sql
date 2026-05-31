-- Закладки сборов: пользователь сохраняет сбор «на потом».
CREATE TABLE sbor_bookmarks (
    user_id  UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    sbor_id  UUID NOT NULL REFERENCES sbory(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, sbor_id)
);

CREATE INDEX idx_sbor_bookmarks_user ON sbor_bookmarks(user_id, created_at DESC);
