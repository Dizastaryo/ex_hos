-- PERF-001: Full-text / trigram search index on chat messages.
-- pg_trgm allows ILIKE '%query%' to use the index (no schema changes needed).
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_messages_text_trgm
    ON messages USING gin (text gin_trgm_ops)
    WHERE text IS NOT NULL AND text <> '';
