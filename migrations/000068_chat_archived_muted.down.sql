ALTER TABLE conversation_participants
    DROP COLUMN IF EXISTS archived_at,
    DROP COLUMN IF EXISTS muted;
