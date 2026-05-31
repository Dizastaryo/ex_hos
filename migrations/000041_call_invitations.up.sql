-- C-1 история звонков + C-8 missed-call notification.
--
-- call_invitations записывается на каждый `call.invite`:
--   - pending  — invite отправлен, peer ещё не ответил
--   - accepted — peer принял (`call.accept`), accepted_at заполнен
--   - declined — peer отказался (`call.decline`)
--   - missed   — peer не ответил пока caller не сбросил (`call.end` пока pending)
--   - ended    — звонок успешно завершён (был accepted, потом end)
--
-- Frontend читает list через `GET /me/calls` (свежие сверху).
-- C-8: при переходе pending→missed бэк создаёт notification 'missed_call'
-- для callee, чтобы он узнал что ему звонили.

CREATE TABLE IF NOT EXISTS call_invitations (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    from_user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    to_user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind             TEXT NOT NULL CHECK (kind IN ('video','voice')),
    status           TEXT NOT NULL DEFAULT 'pending'
                          CHECK (status IN ('pending','accepted','declined','missed','ended')),
    started_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    accepted_at      TIMESTAMPTZ,
    ended_at         TIMESTAMPTZ,
    duration_seconds INT
);

-- Для GET /me/calls (history per-user, обе стороны).
CREATE INDEX IF NOT EXISTS idx_call_invitations_from_started
    ON call_invitations(from_user_id, started_at DESC);
CREATE INDEX IF NOT EXISTS idx_call_invitations_to_started
    ON call_invitations(to_user_id, started_at DESC);
-- Для UPDATE matching latest pending pair (call.accept/decline).
CREATE INDEX IF NOT EXISTS idx_call_invitations_pair_status
    ON call_invitations(from_user_id, to_user_id, status, started_at DESC);
