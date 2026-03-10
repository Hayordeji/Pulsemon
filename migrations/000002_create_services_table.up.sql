CREATE TABLE IF NOT EXISTS services (
    id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id           UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name              VARCHAR(255) NOT NULL,
    url               VARCHAR(2048) NOT NULL,
    interval          VARCHAR(10) NOT NULL CHECK (interval IN ('30s', '1m', '5m', '10m', '30m')),
    timeout_seconds   INTEGER     NOT NULL,
    expected_status   INTEGER     NOT NULL,
    sla_target        DOUBLE PRECISION NOT NULL,
    is_active         BOOLEAN     NOT NULL DEFAULT true,

    -- Live State
    current_status    VARCHAR(10) NOT NULL DEFAULT 'unknown' CHECK (current_status IN ('up', 'down', 'unknown')),
    last_checked_at   TIMESTAMPTZ,
    failure_streak    INTEGER     NOT NULL DEFAULT 0,

    -- Pre-calculated Latency Stats
    avg_latency_ms    DOUBLE PRECISION NOT NULL DEFAULT 0,
    p95_latency_ms    DOUBLE PRECISION NOT NULL DEFAULT 0,
    p99_latency_ms    DOUBLE PRECISION NOT NULL DEFAULT 0,

    -- SLA Tracking
    sla_percentage    DOUBLE PRECISION NOT NULL DEFAULT 100.0,
    total_checks      INTEGER     NOT NULL DEFAULT 0,
    successful_checks INTEGER     NOT NULL DEFAULT 0,

    -- SSL Certificate Info (HTTPS only)
    ssl_cert_expiry    TIMESTAMPTZ,
    ssl_cert_issuer    VARCHAR(255),
    ssl_cert_valid     BOOLEAN,
    ssl_days_remaining INTEGER,

    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_services_user_active ON services (user_id, is_active);
