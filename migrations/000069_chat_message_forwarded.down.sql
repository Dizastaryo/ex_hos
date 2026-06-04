ALTER TABLE messages
  DROP COLUMN IF EXISTS forwarded_from_message_id,
  DROP COLUMN IF EXISTS forwarded_from_sender;
