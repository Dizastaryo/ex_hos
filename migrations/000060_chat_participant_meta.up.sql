ALTER TABLE conversation_participants ADD COLUMN IF NOT EXISTS pinned_at TIMESTAMPTZ NULL;
ALTER TABLE conversation_participants ADD COLUMN IF NOT EXISTS hidden_at TIMESTAMPTZ NULL;
