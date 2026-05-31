ALTER TABLE conversations
    DROP CONSTRAINT IF EXISTS conversations_kind_check;
ALTER TABLE conversation_participants
    DROP CONSTRAINT IF EXISTS conversation_participants_role_check;

DROP INDEX IF EXISTS idx_conversations_kind_group;

ALTER TABLE conversations
    DROP COLUMN IF EXISTS kind,
    DROP COLUMN IF EXISTS title,
    DROP COLUMN IF EXISTS cover_url,
    DROP COLUMN IF EXISTS created_by;

ALTER TABLE conversation_participants
    DROP COLUMN IF EXISTS role,
    DROP COLUMN IF EXISTS joined_at;
