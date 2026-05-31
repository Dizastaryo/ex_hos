-- CHAT-10.1: «delivered» intermediate state для chat read-receipts.
--
-- Когда сообщение успешно доставлено peer'у через WebSocket (peer был online
-- в момент fan-out'а), бэк выставляет delivered_at = NOW(). Frontend читает
-- is_delivered = (delivered_at IS NOT NULL) и рендерит:
--   ✓        — sent (delivered_at = NULL, is_read = false)
--   ✓✓ серый — delivered (delivered_at != NULL, is_read = false)
--   ✓✓ orange — read (is_read = true; delivered_at статус не важен)
--
-- На существующие сообщения NULL — для них клиент покажет либо «sent»
-- (если !is_read) либо «read» (если уже is_read).

ALTER TABLE messages
    ADD COLUMN IF NOT EXISTS delivered_at TIMESTAMPTZ;

-- Partial index для быстрого lookup'а undelivered-сообщений конкретного
-- chat'а (если когда-то будем делать «запомнить и доставить при online»
-- — но не сейчас). Минимальный размер за счёт WHERE.
CREATE INDEX IF NOT EXISTS idx_messages_undelivered
    ON messages(conversation_id, sender_id)
    WHERE delivered_at IS NULL;
