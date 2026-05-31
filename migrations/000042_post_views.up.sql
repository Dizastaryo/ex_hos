-- FEED-5: трекинг просмотренных постов для де-дупликации в feed'е.
--
-- Frontend после ~5 сек просмотра поста в viewport'е делает POST /posts/:id/view.
-- В feed query фильтруем (или sort'им вниз) уже-viewed-посты — юзер не видит
-- повторно то что уже scroll'ил.
--
-- views_count в `posts` (если когда-то добавим) — это **общий** view-count;
-- post_views — per-viewer запись для personal feed-dedup.

CREATE TABLE IF NOT EXISTS post_views (
    post_id    UUID NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    viewed_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (post_id, user_id)
);

-- Index для feed-query: `WHERE NOT EXISTS (SELECT 1 FROM post_views ...)`.
CREATE INDEX IF NOT EXISTS idx_post_views_user_post
    ON post_views(user_id, post_id);
