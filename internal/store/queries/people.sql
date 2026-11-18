-- name: CountPeople :one
SELECT count(*) FROM people;

-- name: InsertPerson :one
INSERT INTO people (login, password_hash, must_change_password, created_at)
VALUES (?, ?, ?, ?)
RETURNING *;

-- name: GetPersonByLogin :one
SELECT * FROM people WHERE login = ?;

-- name: GetPerson :one
SELECT * FROM people WHERE id = ?;

-- name: GetFirstPerson :one
SELECT * FROM people ORDER BY id LIMIT 1;

-- name: UpdatePassword :execrows
UPDATE people SET password_hash = ?, must_change_password = ? WHERE id = ?;
