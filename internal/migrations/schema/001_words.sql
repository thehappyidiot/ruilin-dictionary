-- +goose Up
CREATE TABLE words (
    id       SERIAL PRIMARY KEY,
    word     TEXT NOT NULL,
    type     TEXT NOT NULL,
    meaning  TEXT NOT NULL,
    sentence TEXT NOT NULL
);

-- +goose Down
DROP TABLE words;
