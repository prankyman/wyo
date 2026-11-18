-- name: CountSecrets :one
SELECT count(*) FROM secrets;

-- name: PutSecret :exec
INSERT INTO secrets (name, encrypted_value, last_four) VALUES (?, ?, ?)
ON CONFLICT (name) DO UPDATE SET encrypted_value = excluded.encrypted_value, last_four = excluded.last_four;

-- name: GetSecret :one
SELECT * FROM secrets WHERE name = ?;

-- name: ListSecretNames :many
SELECT name, last_four FROM secrets ORDER BY name;

-- name: DeleteSecret :execrows
DELETE FROM secrets WHERE name = ?;
