-- +goose Up

CREATE TABLE items (
    id            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    name          VARCHAR(255) NOT NULL,
    description   TEXT,
    category      VARCHAR(100) NOT NULL,
    brand         VARCHAR(100),
    condition     VARCHAR(50)  NOT NULL DEFAULT 'new'
                  CHECK (condition IN ('new', 'open-box', 'used')),
    billing_model VARCHAR(50)  NOT NULL DEFAULT 'one-time'
                  CHECK (billing_model IN ('one-time', 'monthly-rental')),
    deposit_amount NUMERIC(10,2) NOT NULL DEFAULT 50.00,
    price         NUMERIC(10,2) NOT NULL,
    status        VARCHAR(50)  NOT NULL DEFAULT 'draft'
                  CHECK (status IN ('draft', 'published', 'unpublished')),
    location_id   UUID         REFERENCES locations(id),
    created_by    UUID         REFERENCES users(id),
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    version       INTEGER      NOT NULL DEFAULT 1
);

CREATE INDEX idx_items_category    ON items (category);
CREATE INDEX idx_items_status      ON items (status);
CREATE INDEX idx_items_location_id ON items (location_id);

CREATE TABLE item_availability_windows (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    item_id    UUID        NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    starts_at  TIMESTAMPTZ NOT NULL,
    ends_at    TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_window_order CHECK (ends_at > starts_at)
);

CREATE INDEX idx_avail_windows_item_id ON item_availability_windows (item_id);

-- +goose Down
DROP TABLE IF EXISTS item_availability_windows;
DROP TABLE IF EXISTS items;
