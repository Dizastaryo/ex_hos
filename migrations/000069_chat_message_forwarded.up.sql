ALTER TABLE messages
  ADD COLUMN forwarded_from_message_id UUID REFERENCES messages(id) ON DELETE SET NULL,
  ADD COLUMN forwarded_from_sender TEXT NOT NULL DEFAULT '';
