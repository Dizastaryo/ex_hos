-- BACK-5: composite index для comments-by-post listing (хвост ORDER BY created_at DESC).
-- Существующий idx_comments_post_id (только post_id) не покрывает sort — Postgres
-- делает либо sort после, либо sequential scan. Composite cover index ускоряет.
--
-- Остальные индексы из спеки уже существуют:
--   idx_messages_conversation (conversation_id, created_at DESC) — 000005
--   idx_likes_entity (entity_id, entity_type) — 000002
--   idx_notifications_user_id (user_id, is_read, created_at DESC) — 000002
CREATE INDEX IF NOT EXISTS idx_comments_post_created
    ON comments(post_id, created_at DESC)
    WHERE parent_id IS NULL;
