-- name: GetWordByID :one
SELECT * FROM words WHERE id = $1;

-- name: GetRandomWord :one
SELECT * FROM words ORDER BY RANDOM() LIMIT 1;

-- name: SearchWords :many
SELECT * FROM words WHERE word ILIKE '%' || $1 || '%' ORDER BY word;

-- name: CreateWord :one
INSERT INTO words (word, type, meaning, sentence, origin)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: UpdateWord :one
UPDATE words
SET word = $2,
    type = $3,
    meaning = $4,
    sentence = $5,
    origin = $6
WHERE id = $1
RETURNING *;

-- name: GetWordConfusions :many
SELECT w.* FROM words w
JOIN word_confusions wc ON w.id = wc.confused_with_id
WHERE wc.word_id = $1
ORDER BY w.word;
