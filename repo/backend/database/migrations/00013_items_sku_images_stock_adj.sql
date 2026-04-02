-- +goose Up

ALTER TABLE items ADD COLUMN sku VARCHAR(100) UNIQUE;
ALTER TABLE items ADD COLUMN images TEXT[] NOT NULL DEFAULT '{}';

CREATE INDEX idx_items_sku ON items (sku);

CREATE TABLE stock_adjustments (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    item_id         UUID        NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    location_id     UUID        NOT NULL REFERENCES locations(id),
    quantity_change  INTEGER    NOT NULL,
    previous_on_hand INTEGER   NOT NULL,
    new_on_hand      INTEGER   NOT NULL,
    reason_code     VARCHAR(50) NOT NULL
                    CHECK (reason_code IN (
                        'damaged', 'found', 'correction', 'return',
                        'theft', 'audit', 'expired', 'other'
                    )),
    notes           TEXT,
    adjusted_by     UUID        REFERENCES users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_stock_adj_item_id ON stock_adjustments (item_id);
CREATE INDEX idx_stock_adj_location_id ON stock_adjustments (location_id);
CREATE INDEX idx_stock_adj_created_at ON stock_adjustments (created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS stock_adjustments;
ALTER TABLE items DROP COLUMN IF EXISTS images;
ALTER TABLE items DROP COLUMN IF EXISTS sku;
