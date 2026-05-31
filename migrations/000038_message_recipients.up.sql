-- CHAT-10.2 + CHAT-10.3: per-recipient delivery/read tracking для group-чатов.
--
-- До этой миграции у нас были только `messages.is_read` (bool) и
-- `messages.delivered_at` (timestamp). Они работали для direct-чатов
-- («peer прочитал» = «прочитали»), но в group: 4 участника — если ОДИН
-- открыл чат, `messages.is_read = true` ставился сразу для ВСЕХ исходящих
-- сообщений sender'а. Это давало sender'у ложный сигнал «все прочитали».
--
-- Новая таблица — row per (message_id, user_id) для каждого не-sender'а.
-- Frontend читает `delivered_count` / `read_count` / `recipients_count`
-- и в group-bubble рисует «X из N прочитали».
--
-- Legacy `messages.is_read` / `messages.delivered_at` остаются — service
-- слой обновляет их как «first peer satisfied» для backward compat.
-- GetMessages computes is_read/is_delivered from recipients (см. repo).

CREATE TABLE IF NOT EXISTS message_recipients (
    message_id    UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    user_id       UUID NOT NULL REFERENCES users(id)    ON DELETE CASCADE,
    delivered_at  TIMESTAMPTZ,
    read_at       TIMESTAMPTZ,
    PRIMARY KEY (message_id, user_id)
);

-- Для CHAT-10.3 (late-delivered replay): на user WS Register мы сканируем
-- все undelivered записи и эмиттим `chat.delivered` к отправителям.
-- Partial-index по user_id where delivered_at IS NULL — только нужные строки.
CREATE INDEX IF NOT EXISTS idx_message_recipients_undelivered_user
    ON message_recipients(user_id)
    WHERE delivered_at IS NULL;

-- Для quick read-counts по conversation (нет смысла индексировать read_at
-- because aggregate-only — но nice if needed). Skip пока, добавим если
-- профилирование покажет slow.

-- Bootstrap existing данных: для каждого сообщения создаём recipients для
-- всех participants кроме sender. delivered_at + read_at — NULL (best
-- compromise: пусть UI покажет «sent / 0 read» для старых сообщений,
-- consistent state без guess-work).
INSERT INTO message_recipients (message_id, user_id)
SELECT m.id, cp.user_id
FROM messages m
JOIN conversation_participants cp ON cp.conversation_id = m.conversation_id
WHERE cp.user_id != m.sender_id
ON CONFLICT DO NOTHING;
