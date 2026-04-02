-- +goose Up

CREATE TABLE members (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id          UUID        UNIQUE NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    location_id      UUID        REFERENCES locations(id),
    membership_type  VARCHAR(100) NOT NULL DEFAULT 'standard',
    membership_start DATE,
    membership_end   DATE,
    status           VARCHAR(50)  NOT NULL DEFAULT 'active'
                     CHECK (status IN ('active', 'inactive', 'expired', 'cancelled')),
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    version          INTEGER      NOT NULL DEFAULT 1
);

CREATE INDEX idx_members_user_id     ON members (user_id);
CREATE INDEX idx_members_location_id ON members (location_id);
CREATE INDEX idx_members_status      ON members (status);

CREATE TABLE coaches (
    id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID         UNIQUE NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    location_id UUID         REFERENCES locations(id),
    specialties TEXT[]       NOT NULL DEFAULT '{}',
    bio         TEXT,
    is_active   BOOLEAN      NOT NULL DEFAULT true,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_coaches_user_id     ON coaches (user_id);
CREATE INDEX idx_coaches_location_id ON coaches (location_id);

-- +goose Down
DROP TABLE IF EXISTS coaches;
DROP TABLE IF EXISTS members;
