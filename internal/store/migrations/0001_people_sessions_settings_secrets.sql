-- The first migration: who may sign in, their sessions, the settings and the secret store. (The table that records
-- the migrations, schema_migrations, is made by the program before any migration: schema_migrations.sql.)
-- Times are whole seconds since 1970-01-01 UTC. The tables are STRICT: a value of the wrong kind is refused by the
-- database itself.

-- The owner: the one person who signs in. The wyo server creates a row only when the table is empty.
CREATE TABLE people (
    id                   INTEGER PRIMARY KEY,
    login                TEXT    NOT NULL UNIQUE,
    password_hash        TEXT    NOT NULL,
    must_change_password INTEGER NOT NULL DEFAULT 0 CHECK (must_change_password IN (0, 1)),
    created_at           INTEGER NOT NULL
) STRICT;

-- A session keeps only the SHA-256 of the text in the browser's cookie, never the text.
CREATE TABLE sessions (
    token_hash   BLOB    PRIMARY KEY CHECK (length(token_hash) = 32),
    person_id    INTEGER NOT NULL REFERENCES people (id) ON DELETE CASCADE,
    created_at   INTEGER NOT NULL,
    active_until INTEGER NOT NULL,
    browser      TEXT    NOT NULL
) STRICT;

CREATE INDEX sessions_person_id ON sessions (person_id);

CREATE TABLE settings (
    key        TEXT    PRIMARY KEY,
    value      TEXT    NOT NULL,
    changed_at INTEGER NOT NULL
) STRICT;

-- Marketplace keys and the like, encrypted with the key file. last_four is what the interface may show of a value.
CREATE TABLE secrets (
    name            TEXT PRIMARY KEY,
    encrypted_value BLOB NOT NULL,
    last_four       TEXT NOT NULL
) STRICT;
