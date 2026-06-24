-- No-op: 000112 owns the DROP for live_streams / live_stream_viewers.
-- This migration only heals environments where 000112 failed to create them.
SELECT 1;
