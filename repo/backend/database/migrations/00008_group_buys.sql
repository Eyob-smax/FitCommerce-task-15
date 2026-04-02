-- +goose Up

CREATE TABLE group_buys (
    id               UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    item_id          UUID         NOT NULL REFERENCES items(id),
    location_id      UUID         NOT NULL REFERENCES locations(id),
    created_by       UUID         REFERENCES users(id),
    title            VARCHAR(255) NOT NULL,
    description      TEXT,
    min_quantity     INTEGER      NOT NULL CHECK (min_quantity > 0),
    current_quantity INTEGER      NOT NULL DEFAULT 0 CHECK (current_quantity >= 0),
    status           VARCHAR(50)  NOT NULL DEFAULT 'draft'
                     CHECK (status IN (
                         'draft', 'published', 'active',
                         'succeeded', 'failed', 'cancelled', 'fulfilled'
                     )),
    cutoff_at        TIMESTAMPTZ  NOT NULL,
    price_per_unit   NUMERIC(10,2) NOT NULL,
    notes            TEXT,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    version          INTEGER      NOT NULL DEFAULT 1
);

CREATE INDEX idx_group_buys_item_id     ON group_buys (item_id);
CREATE INDEX idx_group_buys_location_id ON group_buys (location_id);
CREATE INDEX idx_group_buys_status      ON group_buys (status);
CREATE INDEX idx_group_buys_cutoff_at   ON group_buys (cutoff_at);

CREATE TABLE group_buy_participants (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    group_buy_id  UUID        NOT NULL REFERENCES group_buys(id) ON DELETE CASCADE,
    member_id     UUID        NOT NULL REFERENCES members(id),
    quantity      INTEGER     NOT NULL DEFAULT 1 CHECK (quantity > 0),
    joined_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    status        VARCHAR(50)  NOT NULL DEFAULT 'committed'
                  CHECK (status IN ('committed', 'cancelled')),
    UNIQUE (group_buy_id, member_id)
);

CREATE INDEX idx_gb_participants_group_buy_id ON group_buy_participants (group_buy_id);
CREATE INDEX idx_gb_participants_member_id    ON group_buy_participants (member_id);

-- +goose Down
DROP TABLE IF EXISTS group_buy_participants;
DROP TABLE IF EXISTS group_buys;
