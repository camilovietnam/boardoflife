-- +goose Up
-- Optional metadata picked from a public book catalogue. Progress stays a
-- percentage, so no page counts are stored (editions differ).
ALTER TABLE books
    ADD COLUMN cover_url   TEXT CHECK (char_length(cover_url)   <= 2000),
    ADD COLUMN description TEXT CHECK (char_length(description) <= 5000),
    ADD COLUMN external_id TEXT CHECK (char_length(external_id) <= 100);

-- +goose Down
ALTER TABLE books
    DROP COLUMN external_id,
    DROP COLUMN description,
    DROP COLUMN cover_url;
