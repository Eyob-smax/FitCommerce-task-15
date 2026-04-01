-- +goose Up

CREATE TABLE inventory_stock (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    item_id     UUID        NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    location_id UUID        NOT NULL REFERENCES locations(id),
    on_hand     INTEGER     NOT NULL DEFAULT 0 CHECK (on_hand >= 0),
    reserved    INTEGER     NOT NULL DEFAULT 0 CHECK (reserved >= 0),
    allocated   INTEGER     NOT NULL DEFAULT 0 CHECK (allocated >= 0),
    in_rental   INTEGER     NOT NULL DEFAULT 0 CHECK (in_rental >= 0),
    returned    INTEGER     NOT NULL DEFAULT 0 CHECK (returned >= 0),
    damaged     INTEGER     NOT NULL DEFAULT 0 CHECK (damaged >= 0),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (item_id, location_id)
);

CREATE INDEX idx_stock_item_id     ON inventory_stock (item_id);
CREATE INDEX idx_stock_location_id ON inventory_stock (location_id);

-- available = on_hand - reserved - allocated
-- This is a computed value, not stored; calculated in queries.

-- +goose Down
DROP TABLE IF EXISTS inventory_stock;
