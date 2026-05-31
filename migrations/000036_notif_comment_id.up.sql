-- comment_id для deep-link к конкретному комментарию в нотификациях.
-- Используется только для type='comment' / 'reply' / 'mention'.
-- ON DELETE SET NULL — если комментарий удалили, notification остаётся
-- но без deep-link'а (фронт fallback'ит к /post/:postId).
ALTER TABLE notifications
    ADD COLUMN IF NOT EXISTS comment_id UUID
        REFERENCES comments(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_notifications_comment_id ON notifications(comment_id)
    WHERE comment_id IS NOT NULL;
