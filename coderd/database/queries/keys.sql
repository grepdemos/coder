-- name: GetKeys :many
SELECT *
FROM keys
WHERE secret IS NOT NULL;

-- name: GetKeyByFeatureAndSequence :one
SELECT *
FROM keys
WHERE feature = $1
  AND sequence = $2
  AND secret IS NOT NULL
  AND $3 >= starts_at
  AND ($3 < deletes_at OR deletes_at IS NULL);

-- name: DeleteKey :exec
UPDATE keys
SET secret = NULL
WHERE feature = $1 AND sequence = $2;


-- name: InsertKey :exec
INSERT INTO keys (
    feature,
    sequence,
    secret,
    starts_at
) VALUES (
    $1,
    $2,
    $3,
    $4
);

-- name: UpdateKeyDeletesAt :exec
UPDATE keys
SET deletes_at = $3
WHERE feature = $1 AND sequence = $2;










