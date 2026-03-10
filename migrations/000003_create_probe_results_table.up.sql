CREATE TABLE IF NOT EXISTS probe_results (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    service_id    UUID        NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    user_id       UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status_code   INTEGER     NOT NULL,
    latency_ms    DOUBLE PRECISION NOT NULL,
    is_success    BOOLEAN     NOT NULL,
    error_message TEXT,
    checked_at    TIMESTAMPTZ NOT NULL,
    cert_expiry   TIMESTAMPTZ,
    cert_valid    BOOLEAN
);

CREATE INDEX IF NOT EXISTS idx_probe_results_service_checked ON probe_results (service_id, checked_at);
CREATE INDEX IF NOT EXISTS idx_probe_results_checked ON probe_results (checked_at);
CREATE INDEX IF NOT EXISTS idx_probe_results_user ON probe_results (user_id);
