-- Per-page reading time tracker: honest reading measurement.
-- seconds_spent accumulates real time the user spent viewing each page.
-- A page is considered "read" when seconds_spent >= 15 (threshold in app code).
CREATE TABLE IF NOT EXISTS page_reading_progress (
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    file_id     UUID NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    page_number INT  NOT NULL,
    seconds_spent INT NOT NULL DEFAULT 0,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, file_id, page_number)
);

CREATE INDEX IF NOT EXISTS idx_prp_user_file
    ON page_reading_progress(user_id, file_id);
