-- +goose Up

-- Immutable system-wide audit trail
CREATE TABLE audit_log (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_id        UUID        REFERENCES users(id),
    action          VARCHAR(100) NOT NULL,
    entity_type     VARCHAR(100) NOT NULL,
    entity_id       UUID        NOT NULL,
    before_snapshot JSONB,
    after_snapshot  JSONB,
    ip_address      VARCHAR(45),
    occurred_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_actor_id    ON audit_log (actor_id);
CREATE INDEX idx_audit_entity      ON audit_log (entity_type, entity_id);
CREATE INDEX idx_audit_occurred_at ON audit_log (occurred_at DESC);

-- KPI snapshots — materialized by the worker process
CREATE TABLE kpi_snapshots (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    location_id  UUID        REFERENCES locations(id),
    granularity  VARCHAR(20)  NOT NULL CHECK (granularity IN ('daily', 'weekly', 'monthly')),
    period_start DATE         NOT NULL,
    period_end   DATE         NOT NULL,
    metric_type  VARCHAR(100) NOT NULL,
    value        NUMERIC(15,4) NOT NULL,
    metadata     JSONB        NOT NULL DEFAULT '{}',
    computed_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE (location_id, granularity, period_start, metric_type)
);

CREATE INDEX idx_kpi_location_id  ON kpi_snapshots (location_id);
CREATE INDEX idx_kpi_metric_type  ON kpi_snapshots (metric_type);
CREATE INDEX idx_kpi_period_start ON kpi_snapshots (period_start);

-- +goose Down
DROP TABLE IF EXISTS kpi_snapshots;
DROP TABLE IF EXISTS audit_log;
