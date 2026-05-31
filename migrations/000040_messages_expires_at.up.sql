-- CHAT-11: disappearing messages с TTL.
--
-- Frontend при send'е может прикрепить `expires_in_seconds` (1ч/24ч/7д), бэк
-- считает absolute time → `messages.expires_at`. Janitor-goroutine в
-- cmd/api/main.go раз в 60 сек DELETE'ит истёкшие. Frontend параллельно
-- держит свой Timer + auto-remove из локального state — UX не зависит от
-- частоты janitor'а.
--
-- GetMessages фильтрует `WHERE expires_at IS NULL OR expires_at > NOW()`
-- — стабильно даже если janitor пропустил тик (например, restart api).

ALTER TABLE messages
    ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ;

-- Partial-index — janitor бьёт по нему `WHERE expires_at <= NOW()`. NULL'ы
-- (большинство сообщений) исключены — index получается компактный.
CREATE INDEX IF NOT EXISTS idx_messages_expires_at
    ON messages(expires_at)
    WHERE expires_at IS NOT NULL;
