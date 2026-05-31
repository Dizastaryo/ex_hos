ALTER TABLE room_participants
    ADD COLUMN IF NOT EXISTS is_admin BOOLEAN NOT NULL DEFAULT false;

-- The creator is always an admin; mark existing creators.
UPDATE room_participants rp
SET is_admin = true
FROM rooms r
WHERE rp.room_id = r.id AND rp.user_id = r.creator_id;
