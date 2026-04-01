-- +goose Up

CREATE TABLE export_jobs (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    report_type VARCHAR(100) NOT NULL,
    format      VARCHAR(10)  NOT NULL CHECK (format IN ('csv', 'pdf')),
    filters     JSONB        NOT NULL DEFAULT '{}',
    status      VARCHAR(50)  NOT NULL DEFAULT 'queued'
                CHECK (status IN ('queued', 'processing', 'completed', 'failed')),
    file_path   VARCHAR(500),
    error_msg   TEXT,
    created_by  UUID        REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_export_jobs_status     ON export_jobs (status);
CREATE INDEX idx_export_jobs_created_by ON export_jobs (created_by);

-- Sync mutation queue for offline-first support
CREATE TABLE sync_mutations (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    idempotency_key  UUID        UNIQUE NOT NULL,
    client_id        VARCHAR(255) NOT NULL,
    entity_type      VARCHAR(100) NOT NULL,
    entity_id        UUID,
    operation        VARCHAR(20)  NOT NULL CHECK (operation IN ('create', 'update', 'delete')),
    payload          JSONB        NOT NULL,
    status           VARCHAR(50)  NOT NULL DEFAULT 'pending'
                     CHECK (status IN ('pending', 'applied', 'rejected', 'conflict')),
    conflict_data    JSONB,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    processed_at     TIMESTAMPTZ
);

CREATE INDEX idx_sync_mutations_idempotency ON sync_mutations (idempotency_key);
CREATE INDEX idx_sync_mutations_status      ON sync_mutations (status);
CREATE INDEX idx_sync_mutations_client_id   ON sync_mutations (client_id);
CREATE INDEX idx_sync_mutations_entity      ON sync_mutations (entity_type, entity_id);

-- +goose Down
DROP TABLE IF EXISTS sync_mutations;
DROP TABLE IF EXISTS export_jobs;
