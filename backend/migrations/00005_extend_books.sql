-- +goose Up
-- Books become per-user records of a reading tracker.
--
-- Pre-existing rows (the table had no owner before) are handled safely:
--   * they are assigned to the lowest user id (normally the bootstrap admin);
--   * if there is no user at all they cannot be owned by anyone and are deleted.
-- Legacy name/author values are also repaired so the new CHECKs can be added.
ALTER TABLE books ADD COLUMN user_id BIGINT REFERENCES users (id) ON DELETE CASCADE;

UPDATE books SET user_id = (SELECT min(id) FROM users);
DELETE FROM books WHERE user_id IS NULL;
ALTER TABLE books ALTER COLUMN user_id SET NOT NULL;

UPDATE books
SET name   = COALESCE(NULLIF(left(btrim(name), 300), ''), 'Untitled'),
    author = COALESCE(NULLIF(left(btrim(author), 300), ''), 'Unknown');

ALTER TABLE books
    ADD COLUMN progress   SMALLINT    NOT NULL DEFAULT 0 CHECK (progress BETWEEN 0 AND 100),
    -- Rating in half-stars: NULL (unrated) or 2..10, i.e. 1 to 5 stars.
    ADD COLUMN rating     SMALLINT    CHECK (rating IS NULL OR rating BETWEEN 2 AND 10),
    ADD COLUMN created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ADD CONSTRAINT books_name_check   CHECK (btrim(name) <> ''   AND char_length(name)   <= 300),
    ADD CONSTRAINT books_author_check CHECK (btrim(author) <> '' AND char_length(author) <= 300);

UPDATE books SET progress = 100 WHERE status = 'finished';

CREATE INDEX books_user_status_created_idx ON books (user_id, status, created_at DESC);

-- +goose Down
DROP INDEX books_user_status_created_idx;

ALTER TABLE books
    DROP CONSTRAINT books_author_check,
    DROP CONSTRAINT books_name_check,
    DROP COLUMN updated_at,
    DROP COLUMN created_at,
    DROP COLUMN rating,
    DROP COLUMN progress,
    DROP COLUMN user_id;
