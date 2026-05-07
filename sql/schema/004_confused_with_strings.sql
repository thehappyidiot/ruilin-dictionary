-- +goose Up
ALTER TABLE words
ADD COLUMN confused_with TEXT[] NOT NULL DEFAULT '{}';

UPDATE words w
SET confused_with = mapped.confused_words
FROM (
    SELECT wc.word_id, ARRAY_AGG(cw.word ORDER BY cw.word) AS confused_words
    FROM word_confusions wc
    JOIN words cw ON cw.id = wc.confused_with_id
    GROUP BY wc.word_id
) mapped
WHERE w.id = mapped.word_id;

DROP TABLE IF EXISTS word_confusions;

-- +goose Down
CREATE TABLE word_confusions (
    word_id          INT NOT NULL REFERENCES words(id) ON DELETE CASCADE,
    confused_with_id INT NOT NULL REFERENCES words(id) ON DELETE CASCADE,
    PRIMARY KEY (word_id, confused_with_id),
    CHECK (word_id <> confused_with_id)
);

ALTER TABLE words
DROP COLUMN confused_with;
