-- BUG-021: When a host account is deleted the sbor was silently cascade-deleted
-- without notifying participants via WS.
--
-- Fix:
--   1. Make host_id nullable so ON DELETE SET NULL preserves the sbory row.
--   2. Add a BEFORE DELETE trigger on users that marks the host's sbory as
--      cancelled BEFORE the FK is set to NULL, and fires pg_notify so the Go
--      app can pick it up and send WS notifications to participants.

-- Step 1: allow NULL on host_id
ALTER TABLE sbory ALTER COLUMN host_id DROP NOT NULL;

-- Step 2: swap the FK constraint from CASCADE to SET NULL
ALTER TABLE sbory DROP CONSTRAINT sbory_host_id_fkey;
ALTER TABLE sbory ADD CONSTRAINT sbory_host_id_fkey
    FOREIGN KEY (host_id) REFERENCES users(id) ON DELETE SET NULL;

-- Step 3: trigger function — cancel active sbory and notify the app
CREATE OR REPLACE FUNCTION fn_cancel_sbory_on_host_delete()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE
    r RECORD;
BEGIN
    FOR r IN
        SELECT id, chat_id FROM sbory
        WHERE host_id = OLD.id AND NOT is_cancelled
    LOOP
        UPDATE sbory SET is_cancelled = true, updated_at = now()
        WHERE id = r.id;

        -- pg_notify lets the Go listener forward a WS push to participants.
        PERFORM pg_notify(
            'sbor_cancelled',
            json_build_object('sbor_id', r.id, 'chat_id', r.chat_id)::text
        );
    END LOOP;
    RETURN OLD;
END;
$$;

CREATE TRIGGER trg_cancel_sbory_on_host_delete
BEFORE DELETE ON users
FOR EACH ROW EXECUTE FUNCTION fn_cancel_sbory_on_host_delete();
