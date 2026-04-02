-- +goose Up

CREATE TABLE orders (
    id             UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    group_buy_id   UUID         REFERENCES group_buys(id),
    member_id      UUID         NOT NULL REFERENCES members(id),
    location_id    UUID         NOT NULL REFERENCES locations(id),
    status         VARCHAR(50)  NOT NULL DEFAULT 'pending'
                   CHECK (status IN (
                       'pending', 'confirmed', 'processing',
                       'fulfilled', 'cancelled', 'refunded'
                   )),
    total_amount   NUMERIC(10,2) NOT NULL,
    deposit_amount NUMERIC(10,2) NOT NULL DEFAULT 0.00,
    notes          TEXT,
    created_by     UUID         REFERENCES users(id),
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    version        INTEGER      NOT NULL DEFAULT 1
);

CREATE INDEX idx_orders_member_id    ON orders (member_id);
CREATE INDEX idx_orders_location_id  ON orders (location_id);
CREATE INDEX idx_orders_status       ON orders (status);
CREATE INDEX idx_orders_group_buy_id ON orders (group_buy_id);

CREATE TABLE order_line_items (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id        UUID        NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    item_id         UUID        NOT NULL REFERENCES items(id),
    quantity        INTEGER     NOT NULL CHECK (quantity > 0),
    unit_price      NUMERIC(10,2) NOT NULL,
    deposit_per_unit NUMERIC(10,2) NOT NULL DEFAULT 0.00
);

CREATE INDEX idx_order_lines_order_id ON order_line_items (order_id);

CREATE TABLE order_notes (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id   UUID        NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    author_id  UUID        REFERENCES users(id),
    content    TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_order_notes_order_id ON order_notes (order_id);

CREATE TABLE order_timeline_events (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id        UUID        NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    actor_id        UUID        REFERENCES users(id),
    event_type      VARCHAR(50) NOT NULL
                    CHECK (event_type IN (
                        'status_change', 'adjustment', 'split',
                        'cancellation', 'note', 'refund', 'creation'
                    )),
    description     TEXT        NOT NULL,
    before_snapshot JSONB,
    after_snapshot  JSONB,
    occurred_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_timeline_order_id    ON order_timeline_events (order_id);
CREATE INDEX idx_timeline_occurred_at ON order_timeline_events (occurred_at);

-- +goose Down
DROP TABLE IF EXISTS order_timeline_events;
DROP TABLE IF EXISTS order_notes;
DROP TABLE IF EXISTS order_line_items;
DROP TABLE IF EXISTS orders;
