CREATE TABLE IF NOT EXISTS alerts (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    service_id  UUID        NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    user_id     UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    alert_type  VARCHAR(20) NOT NULL CHECK (alert_type IN ('failure_streak', 'sla_breach', 'ssl_expiry', 'recovery')),
    message     TEXT        NOT NULL,
    sent_at     TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_alerts_service_sent ON alerts (service_id, sent_at);
CREATE INDEX IF NOT EXISTS idx_alerts_user ON alerts (user_id);
