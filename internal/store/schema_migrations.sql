-- The table that records every migration that was applied, with the checksum of its text at that moment. The wyo
-- server refuses to start when its own copy of an applied migration differs. It is made before the first migration
-- runs (it is not itself a migration, so a migration can always be recorded), and only when it is missing.
-- Times are whole seconds since 1970-01-01 UTC.
CREATE TABLE IF NOT EXISTS schema_migrations (
    version    INTEGER PRIMARY KEY,
    name       TEXT    NOT NULL,
    checksum   TEXT    NOT NULL,
    applied_at INTEGER NOT NULL
) STRICT;
