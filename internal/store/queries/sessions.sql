-- name: InsertSession :exec
INSERT INTO sessions (token_hash, person_id, created_at, active_until, browser) VALUES (?, ?, ?, ?, ?);

-- name: GetSession :one
SELECT * FROM sessions WHERE token_hash = ?;

-- name: ProlongSession :exec
UPDATE sessions SET active_until = ? WHERE token_hash = ?;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE token_hash = ?;

-- name: DeleteSessionsOfPerson :exec
DELETE FROM sessions WHERE person_id = ?;

-- name: DeleteOtherSessionsOfPerson :exec
DELETE FROM sessions WHERE person_id = ? AND token_hash <> ?;

-- name: DeleteEndedSessions :exec
DELETE FROM sessions WHERE active_until <= ?;

-- name: CountSessionsOfPerson :one
SELECT count(*) FROM sessions WHERE person_id = ?;
