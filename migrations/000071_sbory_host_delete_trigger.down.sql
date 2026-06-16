DROP TRIGGER IF EXISTS trg_cancel_sbory_on_host_delete ON users;
DROP FUNCTION IF EXISTS fn_cancel_sbory_on_host_delete();

ALTER TABLE sbory DROP CONSTRAINT sbory_host_id_fkey;
ALTER TABLE sbory ALTER COLUMN host_id SET NOT NULL;
ALTER TABLE sbory ADD CONSTRAINT sbory_host_id_fkey
    FOREIGN KEY (host_id) REFERENCES users(id) ON DELETE CASCADE;
