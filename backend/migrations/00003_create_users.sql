-- +goose Up
CREATE TABLE users (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    username      TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    role_id       BIGINT NOT NULL REFERENCES roles (id),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Usernames are unique regardless of case.
CREATE UNIQUE INDEX users_username_lower_idx ON users (lower(username));

-- +goose Down
DROP TABLE users;
