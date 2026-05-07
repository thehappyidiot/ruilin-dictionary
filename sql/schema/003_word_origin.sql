-- +goose Up
ALTER TABLE words
ADD COLUMN origin TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE words
DROP COLUMN origin;
