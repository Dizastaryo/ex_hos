-- Привязка group-чата к сбору.
-- chat_id заполняется сразу при создании сбора через SborService.Create.
ALTER TABLE sbory ADD COLUMN IF NOT EXISTS chat_id UUID REFERENCES conversations(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_sbory_chat_id ON sbory(chat_id);
