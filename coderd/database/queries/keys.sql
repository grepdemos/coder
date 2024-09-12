-- name: GetCryptoKeys :many
SELECT *
FROM crypto_keys
WHERE secret IS NOT NULL;

-- name: GetLatestCryptoKeyByFeature :one
SELECT *
FROM crypto_keys
WHERE feature = $1
ORDER BY sequence DESC
LIMIT 1;


-- name: GetCryptoKeyByFeatureAndSequence :one
SELECT *
FROM crypto_keys
WHERE feature = $1
  AND sequence = $2
  AND secret IS NOT NULL
  AND @time >= starts_at
  AND (@time < deletes_at OR deletes_at IS NULL);

-- name: DeleteCryptoKey :one
UPDATE crypto_keys
SET secret = NULL
WHERE feature = $1 AND sequence = $2 RETURNING *;

-- name: InsertCryptoKey :one
INSERT INTO crypto_keys (
    feature,
    sequence,
    secret,
    starts_at
) VALUES (
    $1,
    $2,
    $3,
    $4
) RETURNING *;

-- name: UpdateCryptoKeyDeletesAt :one
UPDATE crypto_keys
SET deletes_at = $3
WHERE feature = $1 AND sequence = $2 RETURNING *;










