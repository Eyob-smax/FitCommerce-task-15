-- +goose Up

CREATE TABLE payment_ledger (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id     UUID        NOT NULL REFERENCES orders(id),
    member_id    UUID        NOT NULL REFERENCES members(id),
    amount       NUMERIC(10,2) NOT NULL,
    type         VARCHAR(50)  NOT NULL
                 CHECK (type IN ('charge', 'deposit', 'refund', 'partial_refund')),
    status       VARCHAR(50)  NOT NULL DEFAULT 'pending'
                 CHECK (status IN (
                     'pending', 'authorized', 'captured',
                     'refunded', 'partially_refunded', 'voided'
                 )),
    reference_id VARCHAR(255),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_payment_order_id  ON payment_ledger (order_id);
CREATE INDEX idx_payment_member_id ON payment_ledger (member_id);
CREATE INDEX idx_payment_status    ON payment_ledger (status);

-- +goose Down
DROP TABLE IF EXISTS payment_ledger;
