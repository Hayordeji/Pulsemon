ALTER TABLE users
ADD COLUMN IF NOT EXISTS provider
    VARCHAR(50) NOT NULL DEFAULT 'email';

ALTER TABLE users
ADD COLUMN IF NOT EXISTS google_id
    VARCHAR(255);

-- Postgres unique indexes allow multiple NULLs:
-- only Google-linked accounts are constrained to one per user
CREATE UNIQUE INDEX IF NOT EXISTS uq_users_google_id
    ON users (google_id);
