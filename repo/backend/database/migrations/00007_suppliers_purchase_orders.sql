-- +goose Up

CREATE TABLE suppliers (
    id           UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    name         VARCHAR(255) NOT NULL,
    contact_name VARCHAR(255),
    email        VARCHAR(255),
    phone        VARCHAR(50),
    address      TEXT,
    is_active    BOOLEAN      NOT NULL DEFAULT true,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_suppliers_name ON suppliers (name);

CREATE TABLE purchase_orders (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    supplier_id UUID        NOT NULL REFERENCES suppliers(id),
    location_id UUID        NOT NULL REFERENCES locations(id),
    status      VARCHAR(50)  NOT NULL DEFAULT 'draft'
                CHECK (status IN (
                    'draft', 'issued', 'partially_received',
                    'received', 'cancelled', 'closed'
                )),
    notes       TEXT,
    issued_at   TIMESTAMPTZ,
    expected_at DATE,
    created_by  UUID        REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    version     INTEGER     NOT NULL DEFAULT 1
);

CREATE INDEX idx_po_supplier_id ON purchase_orders (supplier_id);
CREATE INDEX idx_po_location_id ON purchase_orders (location_id);
CREATE INDEX idx_po_status      ON purchase_orders (status);

CREATE TABLE po_line_items (
    id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    po_id             UUID        NOT NULL REFERENCES purchase_orders(id) ON DELETE CASCADE,
    item_id           UUID        NOT NULL REFERENCES items(id),
    quantity          INTEGER     NOT NULL CHECK (quantity > 0),
    unit_cost         NUMERIC(10,2) NOT NULL,
    received_quantity INTEGER     NOT NULL DEFAULT 0 CHECK (received_quantity >= 0)
);

CREATE INDEX idx_po_lines_po_id ON po_line_items (po_id);

CREATE TABLE goods_receipts (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    po_id       UUID        NOT NULL REFERENCES purchase_orders(id),
    received_by UUID        REFERENCES users(id),
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    notes       TEXT
);

CREATE INDEX idx_receipts_po_id ON goods_receipts (po_id);

CREATE TABLE goods_receipt_lines (
    id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    receipt_id        UUID        NOT NULL REFERENCES goods_receipts(id) ON DELETE CASCADE,
    po_line_item_id   UUID        NOT NULL REFERENCES po_line_items(id),
    quantity_received INTEGER     NOT NULL CHECK (quantity_received >= 0),
    discrepancy_notes TEXT
);

CREATE INDEX idx_receipt_lines_receipt_id ON goods_receipt_lines (receipt_id);

-- +goose Down
DROP TABLE IF EXISTS goods_receipt_lines;
DROP TABLE IF EXISTS goods_receipts;
DROP TABLE IF EXISTS po_line_items;
DROP TABLE IF EXISTS purchase_orders;
DROP TABLE IF EXISTS suppliers;
