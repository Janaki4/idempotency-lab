-- One row per user. The balance is what must never be credited twice.
CREATE TABLE wallets (
    user_id TEXT PRIMARY KEY,
    balance BIGINT NOT NULL DEFAULT 0
);

-- One row per job that has already been applied: the "receipt".
-- Nothing uses it yet; the idempotent handler we write together will.
CREATE TABLE processed_jobs (
    job_id       TEXT PRIMARY KEY,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
