-- Pinned message в conversation. Один pinned на чат (если нужно несколько —
-- отдельную таблицу chat_pins, отложено). ON DELETE SET NULL — если pinned
-- message удалили, поле сброшится; UI отрисует «нет закреплённого».

ALTER TABLE conversations
    ADD COLUMN IF NOT EXISTS pinned_message_id UUID
        REFERENCES messages(id) ON DELETE SET NULL;
