CREATE TABLE IF NOT EXISTS roles (
    id         UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    name       VARCHAR(50)  NOT NULL UNIQUE,
    created_at TIMESTAMP    NOT NULL DEFAULT NOW()
);

ALTER TABLE users
ADD COLUMN IF NOT EXISTS role_id UUID REFERENCES roles(id);

ALTER TABLE users
ADD COLUMN IF NOT EXISTS username VARCHAR(50) UNIQUE;
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email ON users (email);
