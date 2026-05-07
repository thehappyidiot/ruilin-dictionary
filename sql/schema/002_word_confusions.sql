-- +goose Up
CREATE TABLE word_confusions (
    word_id          INT NOT NULL REFERENCES words(id) ON DELETE CASCADE,
    confused_with_id INT NOT NULL REFERENCES words(id) ON DELETE CASCADE,
    PRIMARY KEY (word_id, confused_with_id),
    CHECK (word_id <> confused_with_id)
);

-- +goose Down
DROP TABLE word_confusions;
