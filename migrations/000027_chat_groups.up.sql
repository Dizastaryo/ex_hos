-- Group chats — расширение существующей conversations/conversation_participants
-- модели metadata-полями. Direct-чаты (1-1) остаются как 'direct' с двумя
-- participants без role-разграничения; group-чаты — 'group' с title, cover,
-- creator и role per-participant.

ALTER TABLE conversations
    ADD COLUMN IF NOT EXISTS kind VARCHAR(16) NOT NULL DEFAULT 'direct',
    ADD COLUMN IF NOT EXISTS title TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS cover_url TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS created_by UUID REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE conversation_participants
    ADD COLUMN IF NOT EXISTS role VARCHAR(16) NOT NULL DEFAULT 'member',
    ADD COLUMN IF NOT EXISTS joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

-- Партиал-индекс — быстрый list-чатов group-типа.
CREATE INDEX IF NOT EXISTS idx_conversations_kind_group
    ON conversations(updated_at DESC) WHERE kind = 'group';

-- Защита от bad-data на запись.
ALTER TABLE conversations
    DROP CONSTRAINT IF EXISTS conversations_kind_check;
ALTER TABLE conversations
    ADD CONSTRAINT conversations_kind_check CHECK (kind IN ('direct', 'group'));

ALTER TABLE conversation_participants
    DROP CONSTRAINT IF EXISTS conversation_participants_role_check;
ALTER TABLE conversation_participants
    ADD CONSTRAINT conversation_participants_role_check CHECK (role IN ('member', 'admin'));
