-- name: ListSettings :many
SELECT * FROM settings ORDER BY key;

-- name: GetSetting :one
SELECT * FROM settings WHERE key = ?;

-- name: PutSetting :exec
INSERT INTO settings (key, value, changed_at) VALUES (?, ?, ?)
ON CONFLICT (key) DO UPDATE SET value = excluded.value, changed_at = excluded.changed_at;

-- name: PutSettingIfAbsent :exec
INSERT INTO settings (key, value, changed_at) VALUES (?, ?, ?) ON CONFLICT (key) DO NOTHING;
