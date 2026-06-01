-- Заявки на вступление в сбор.
-- Участник подаёт заявку → организатор принимает/отклоняет.
-- После одобрения пользователь добавляется в sbor_members и group-чат.
CREATE TABLE sbor_requests (
    id         UUID    PRIMARY KEY DEFAULT gen_random_uuid(),
    sbor_id    UUID    NOT NULL REFERENCES sbory(id) ON DELETE CASCADE,
    user_id    UUID    NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status     TEXT    NOT NULL DEFAULT 'pending'
               CHECK (status IN ('pending', 'approved', 'rejected')),
    message    TEXT    NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (sbor_id, user_id)
);
CREATE INDEX idx_sbor_requests_sbor_status ON sbor_requests(sbor_id, status);
CREATE INDEX idx_sbor_requests_user        ON sbor_requests(user_id);
