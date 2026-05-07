-- name: GetWordByID :one
SELECT * FROM words WHERE id = $1;

-- name: GetRandomWord :one
SELECT * FROM words ORDER BY RANDOM() LIMIT 1;

-- name: SearchWords :many
SELECT * FROM words WHERE word ILIKE '%' || $1 || '%' ORDER BY word;

-- name: GetWordConfusions :many
SELECT w.* FROM words w
JOIN word_confusions wc ON w.id = wc.confused_with_id
WHERE wc.word_id = $1
ORDER BY w.word;
