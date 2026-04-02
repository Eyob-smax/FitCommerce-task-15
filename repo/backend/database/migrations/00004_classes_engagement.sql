-- +goose Up

CREATE TABLE classes (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    coach_id         UUID        NOT NULL REFERENCES coaches(id),
    location_id      UUID        NOT NULL REFERENCES locations(id),
    name             VARCHAR(255) NOT NULL,
    description      TEXT,
    scheduled_at     TIMESTAMPTZ  NOT NULL,
    duration_minutes INTEGER      NOT NULL DEFAULT 60,
    capacity         INTEGER      NOT NULL,
    booked_seats     INTEGER      NOT NULL DEFAULT 0,
    status           VARCHAR(50)  NOT NULL DEFAULT 'scheduled'
                     CHECK (status IN ('scheduled', 'cancelled', 'completed')),
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_classes_coach_id    ON classes (coach_id);
CREATE INDEX idx_classes_location_id ON classes (location_id);
CREATE INDEX idx_classes_scheduled   ON classes (scheduled_at);
CREATE INDEX idx_classes_status      ON classes (status);

CREATE TABLE class_bookings (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    class_id   UUID        NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
    member_id  UUID        NOT NULL REFERENCES members(id) ON DELETE CASCADE,
    status     VARCHAR(50)  NOT NULL DEFAULT 'confirmed'
               CHECK (status IN ('confirmed', 'cancelled', 'attended', 'no_show')),
    booked_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE (class_id, member_id)
);

CREATE INDEX idx_class_bookings_class_id  ON class_bookings (class_id);
CREATE INDEX idx_class_bookings_member_id ON class_bookings (member_id);

CREATE TABLE engagement_events (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    member_id   UUID        NOT NULL REFERENCES members(id) ON DELETE CASCADE,
    event_type  VARCHAR(50)  NOT NULL
                CHECK (event_type IN ('attendance', 'order', 'group_buy_join', 'class_booking')),
    entity_id   UUID,
    occurred_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_engagement_member_id   ON engagement_events (member_id);
CREATE INDEX idx_engagement_event_type  ON engagement_events (event_type);
CREATE INDEX idx_engagement_occurred_at ON engagement_events (occurred_at);

-- +goose Down
DROP TABLE IF EXISTS engagement_events;
DROP TABLE IF EXISTS class_bookings;
DROP TABLE IF EXISTS classes;
