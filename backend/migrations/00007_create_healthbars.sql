-- +goose Up
-- One health bar per user and life area ("section").
--
-- health is stored in HALF-POINTS: 0..20 represents 0 to 10 in steps of 0.5
-- (Postgres has no TINYINT, so SMALLINT is the smallest integer type). The
-- API converts to and from points.
--
-- last_updated is when health last changed through the API; a future decay
-- job can compare it with now() to decide how much health to take away.
--
-- The allowed sections are deliberately NOT a CHECK constraint: the Go
-- whitelist is the single source of truth, so adding a life area needs no
-- migration.
CREATE TABLE healthbars (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id      BIGINT      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    section      VARCHAR(32) NOT NULL,
    health       SMALLINT    NOT NULL CHECK (health BETWEEN 0 AND 20),
    last_updated TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, section)
);

-- +goose Down
DROP TABLE healthbars;
