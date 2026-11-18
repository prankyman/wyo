-- name: InsertAccount :one
INSERT INTO marketplace_accounts (
    marketplace, name, state, test, seller_name, seller_profile_id, secret_name, key_last4, key_type, key_expires_at,
    categories, missing_categories, read_only, deadline_hours, last_check_at, last_check_outcome, last_success_at,
    created_at, key_fields
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetAccount :one
SELECT * FROM marketplace_accounts WHERE id = ?;

-- name: ListAccounts :many
SELECT * FROM marketplace_accounts ORDER BY id;

-- name: UpdateAccountSettings :execrows
UPDATE marketplace_accounts SET name = ?, deadline_hours = ? WHERE id = ?;

-- name: UpdateAccountKey :execrows
UPDATE marketplace_accounts
SET state = ?, test = ?, seller_name = ?, seller_profile_id = ?, key_last4 = ?, key_fields = ?, key_type = ?,
    key_expires_at = ?, categories = ?, missing_categories = ?, read_only = ?, last_check_at = ?, last_check_outcome = ?,
    last_success_at = ?, last_refusal_code = '', last_refusal_status = 0, last_refusal_at = NULL, failures_in_row = 0
WHERE id = ?;

-- name: UpdateAccountState :execrows
UPDATE marketplace_accounts SET state = ? WHERE id = ?;

-- name: RecordAccountCheck :execrows
UPDATE marketplace_accounts SET last_check_at = ?, last_check_outcome = ? WHERE id = ?;

-- name: RecordAccountSuccess :execrows
-- A success moves the state only of an account that is being read: never one switched off or with a refused key.
UPDATE marketplace_accounts
SET state = CASE WHEN state IN ('working', 'key_expiring', 'no_connection') THEN sqlc.arg(state) ELSE state END,
    last_success_at = sqlc.arg(last_success_at), failures_in_row = 0
WHERE id = sqlc.arg(id);

-- name: MarkAccountWithoutConnection :execrows
UPDATE marketplace_accounts SET state = 'no_connection' WHERE id = ? AND state IN ('working', 'key_expiring');

-- name: RecordAccountFailure :one
UPDATE marketplace_accounts SET failures_in_row = failures_in_row + 1 WHERE id = ? RETURNING failures_in_row;

-- name: RecordAccountRefusal :execrows
-- An empty state leaves the state as it is, and an account switched off stays switched off.
UPDATE marketplace_accounts
SET state = CASE WHEN state = 'disabled' OR sqlc.arg(state) = '' THEN state ELSE sqlc.arg(state) END,
    last_refusal_code = sqlc.arg(last_refusal_code), last_refusal_status = sqlc.arg(last_refusal_status),
    last_refusal_at = sqlc.arg(last_refusal_at)
WHERE id = sqlc.arg(id);

-- name: DeleteAccount :execrows
DELETE FROM marketplace_accounts WHERE id = ?;

-- name: CountOrdersOfAccount :one
SELECT count(*) FROM orders WHERE account_id = ?;

-- name: DeleteWarehousesOfAccount :exec
DELETE FROM marketplace_warehouses WHERE account_id = ?;

-- name: InsertWarehouse :exec
INSERT INTO marketplace_warehouses (account_id, number, name, seller_delivers, label_type, auto_assembly, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: ListWarehouses :many
SELECT * FROM marketplace_warehouses WHERE account_id = ? ORDER BY name, number;

-- name: GetMark :one
SELECT mark FROM sync_marks WHERE account_id = ? AND feed = ?;

-- name: PutMark :exec
INSERT INTO sync_marks (account_id, feed, mark, changed_at) VALUES (?, ?, ?, ?)
ON CONFLICT (account_id, feed) DO UPDATE SET mark = excluded.mark, changed_at = excluded.changed_at;
