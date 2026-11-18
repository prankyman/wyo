-- name: ListAppliedMigrations :many
SELECT * FROM schema_migrations ORDER BY version;

-- name: InsertAppliedMigration :exec
INSERT INTO schema_migrations (version, name, checksum, applied_at) VALUES (?, ?, ?, ?);
