-- The second migration: the seller's marketplace accounts, their warehouses, the marks of the feeds read, the orders with
-- the history of their states, and the journal of every request sent to a marketplace.
-- Times are whole seconds since 1970-01-01 UTC. The numbers a marketplace gives (an order's number, a warehouse's, a
-- product's) are kept as text: nothing sorts or counts them as numbers. No column holds a buyer's address, a buyer's
-- comment or a buyer's region: the adapters drop them where the answer arrives.

-- One seller profile on one marketplace, with its key. The key itself is in the secret store under secret_name; this row
-- keeps only what the screens may show of it.
CREATE TABLE marketplace_accounts (
    id                  INTEGER PRIMARY KEY,
    marketplace         TEXT    NOT NULL,
    name                TEXT    NOT NULL,
    state               TEXT    NOT NULL CHECK (state IN ('working', 'key_expiring', 'key_expired', 'key_refused',
                                                          'no_connection', 'disabled')),
    test                INTEGER NOT NULL DEFAULT 0 CHECK (test IN (0, 1)),
    seller_name         TEXT    NOT NULL DEFAULT '',
    seller_profile_id   TEXT    NOT NULL DEFAULT '',
    secret_name         TEXT    NOT NULL UNIQUE,
    key_last4           TEXT    NOT NULL,
    key_type            TEXT    NOT NULL DEFAULT '',
    key_expires_at      INTEGER,
    -- The codes of the categories the key opens and of the ones it lacks, separated by spaces.
    categories          TEXT    NOT NULL DEFAULT '',
    missing_categories  TEXT    NOT NULL DEFAULT '',
    read_only           INTEGER NOT NULL DEFAULT 0 CHECK (read_only IN (0, 1)),
    deadline_hours      INTEGER NOT NULL DEFAULT 48 CHECK (deadline_hours BETWEEN 1 AND 720),
    last_check_at       INTEGER,
    last_check_outcome  TEXT    NOT NULL DEFAULT '',
    last_success_at     INTEGER,
    last_refusal_code   TEXT    NOT NULL DEFAULT '',
    last_refusal_status INTEGER NOT NULL DEFAULT 0,
    last_refusal_at     INTEGER,
    failures_in_row     INTEGER NOT NULL DEFAULT 0 CHECK (failures_in_row >= 0),
    created_at          INTEGER NOT NULL
) STRICT;

-- The seller's warehouses on a marketplace, as last read.
CREATE TABLE marketplace_warehouses (
    account_id INTEGER NOT NULL REFERENCES marketplace_accounts (id) ON DELETE CASCADE,
    number     TEXT    NOT NULL,
    name       TEXT    NOT NULL,
    updated_at INTEGER NOT NULL,
    PRIMARY KEY (account_id, number)
) STRICT;

-- How far a feed of an account has been read: the adapter's own text, which it is given back to read on from.
CREATE TABLE sync_marks (
    account_id INTEGER NOT NULL REFERENCES marketplace_accounts (id) ON DELETE CASCADE,
    feed       TEXT    NOT NULL,
    mark       TEXT    NOT NULL,
    changed_at INTEGER NOT NULL,
    PRIMARY KEY (account_id, feed)
) STRICT;

-- One order of one account. join_key is the marketplace's own text that ties a row of its orders feed to the order;
-- barcodes are separated by spaces. The deadline is not kept: it is the creation time plus the account's hours, so a
-- change of the hours moves every deadline at once.
CREATE TABLE orders (
    id                    INTEGER PRIMARY KEY,
    account_id            INTEGER NOT NULL REFERENCES marketplace_accounts (id) ON DELETE RESTRICT,
    number                TEXT    NOT NULL,
    scheme                TEXT    NOT NULL CHECK (scheme IN ('fbs', 'fbo')),
    join_key              TEXT    NOT NULL DEFAULT '',
    basket                TEXT    NOT NULL DEFAULT '',
    warehouse_number      TEXT    NOT NULL DEFAULT '',
    warehouse_name        TEXT    NOT NULL DEFAULT '',
    article               TEXT    NOT NULL DEFAULT '',
    barcodes              TEXT    NOT NULL DEFAULT '',
    product_id            TEXT    NOT NULL DEFAULT '',
    kind                  TEXT    NOT NULL DEFAULT '',
    brand                 TEXT    NOT NULL DEFAULT '',
    size                  TEXT    NOT NULL DEFAULT '',
    created_at            INTEGER NOT NULL,
    state                 TEXT    NOT NULL CHECK (state IN ('new', 'assembling', 'in_delivery', 'received', 'cancelled',
                                                            'unknown')),
    cancelled_by          TEXT    NOT NULL DEFAULT '' CHECK (cancelled_by IN ('', 'seller', 'buyer', 'marketplace')),
    seller_status         TEXT    NOT NULL DEFAULT '',
    marketplace_status    TEXT    NOT NULL DEFAULT '',
    updated_at            INTEGER NOT NULL,
    UNIQUE (account_id, number)
) STRICT;

CREATE INDEX orders_created ON orders (created_at, id);
CREATE INDEX orders_join_key ON orders (account_id, join_key);
CREATE INDEX orders_state ON orders (account_id, scheme, state);

-- Every change of an order's state or of who cancelled it, with where it came from (the contract part, e.g.
-- 'orders.states') and the marketplace's own words at that moment. A change of the words alone is no event.
CREATE TABLE order_events (
    id                 INTEGER PRIMARY KEY,
    order_id           INTEGER NOT NULL REFERENCES orders (id) ON DELETE CASCADE,
    at                 INTEGER NOT NULL,
    state              TEXT    NOT NULL CHECK (state IN ('new', 'assembling', 'in_delivery', 'received', 'cancelled',
                                                         'unknown')),
    cancelled_by       TEXT    NOT NULL DEFAULT '' CHECK (cancelled_by IN ('', 'seller', 'buyer', 'marketplace')),
    seller_status      TEXT    NOT NULL DEFAULT '',
    marketplace_status TEXT    NOT NULL DEFAULT '',
    source             TEXT    NOT NULL
) STRICT;

CREATE INDEX order_events_order ON order_events (order_id, at);

-- One row for every request sent to a marketplace: never a key, never a query, never a body. account_id is no reference:
-- the rows outlive a deleted account, and a row is never changed. path holds placeholders in place of every number.
CREATE TABLE request_journal (
    id          INTEGER PRIMARY KEY,
    at          INTEGER NOT NULL,
    account_id  INTEGER,
    marketplace TEXT    NOT NULL,
    method      TEXT    NOT NULL,
    path        TEXT    NOT NULL,
    status      INTEGER NOT NULL,
    outcome     TEXT    NOT NULL CHECK (outcome IN ('done', 'limited', 'refused', 'server_error', 'no_connection',
                                                    'key_refused')),
    error_code  TEXT    NOT NULL DEFAULT '',
    error_text  TEXT    NOT NULL DEFAULT '' CHECK (length(error_text) <= 500),
    duration_ms INTEGER NOT NULL CHECK (duration_ms >= 0),
    job         TEXT    NOT NULL
) STRICT;

CREATE INDEX request_journal_at ON request_journal (at, id);
CREATE INDEX request_journal_account ON request_journal (account_id, at);

-- The journal takes inserts only. Rows older than 30 days are deleted by the wyo server's nightly job; nothing else
-- deletes them.
CREATE TRIGGER request_journal_insert_only BEFORE UPDATE ON request_journal
BEGIN
    SELECT RAISE(ABORT, 'the request journal takes inserts only');
END;
