-- name: InsertNotification :one
INSERT INTO marketplace_notifications (account_id, at, kind, numbers) VALUES (?, ?, ?, ?)
RETURNING id;

-- name: ListUnreadNotifications :many
SELECT * FROM marketplace_notifications WHERE account_id = ? AND read_at IS NULL ORDER BY id LIMIT ?;

-- name: MarkNotificationsRead :exec
UPDATE marketplace_notifications SET read_at = sqlc.arg(read_at)
WHERE account_id = sqlc.arg(account_id) AND read_at IS NULL AND id <= sqlc.arg(up_to);

-- name: ListAccountsWithUnreadNotifications :many
SELECT DISTINCT account_id FROM marketplace_notifications WHERE read_at IS NULL ORDER BY account_id;

-- name: ListNotifiedNumbersSince :many
SELECT numbers FROM marketplace_notifications WHERE account_id = ? AND at >= ? AND numbers <> '';

-- name: DeleteNotificationsBefore :execrows
DELETE FROM marketplace_notifications WHERE at < ?;
