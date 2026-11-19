-- The seventh migration (Plan 7, Яндекс Маркет): an account's shops, the marketplace's notifications, test orders, an
-- order's own scheme name and Express warehouses. Times are whole seconds since 1970-01-01 UTC. No column holds a key, a
-- buyer's datum or a notification's body.

-- A test order: one the seller made in the marketplace's cabinet to try a connection. It is shown and passed on, marked,
-- and counted in no total (constitution, principle X). Every order kept before this migration is a real one.
ALTER TABLE orders ADD COLUMN test INTEGER NOT NULL DEFAULT 0 CHECK (test IN (0, 1));

-- The marketplace's own name of an order's scheme when it is not the connector's own one: 'express' is an order the seller
-- packs (scheme 'fbs') that a courier takes within hours, 'fby' an order of the marketplace's own warehouse (scheme 'fbo')
-- as Яндекс Маркет names it. The column scheme keeps who packs, which every reading and the exchange go by.
ALTER TABLE orders ADD COLUMN scheme_name TEXT NOT NULL DEFAULT '' CHECK (scheme_name IN ('', 'express', 'fby'));

-- The marketplace's own text of an order's warehouse, kept when the warehouse cannot be told by its number (Яндекс Маркет
-- names a text «в системе магазина» in an order and numbers in its warehouses; which matches is not sourced). Shown in the
-- order's details beside «Склад не определён».
ALTER TABLE orders ADD COLUMN warehouse_text TEXT NOT NULL DEFAULT '';

-- The shops of an account whose marketplace keeps several shops in one cabinet: the marketplace's own number and name of
-- the shop, its scheme in the marketplace's words, and whether the marketplace has switched the shop's API off. An account
-- of a marketplace without shops has no row.
CREATE TABLE marketplace_shops (
    account_id INTEGER NOT NULL REFERENCES marketplace_accounts (id) ON DELETE CASCADE,
    number     TEXT    NOT NULL,
    name       TEXT    NOT NULL,
    scheme     TEXT    NOT NULL DEFAULT '',
    api_off    INTEGER NOT NULL DEFAULT 0 CHECK (api_off IN (0, 1)),
    updated_at INTEGER NOT NULL,
    PRIMARY KEY (account_id, number)
) STRICT;

-- The address text of an account's notifications (the wyo server's address /hooks/marketplace/<hook_text>; '' when its
-- marketplace sends none), when the last notification of the account arrived, and when a safety reading last found an
-- order change that no notification had named. Notifications work while the last one arrived after the last loss.
ALTER TABLE marketplace_accounts ADD COLUMN hook_text TEXT NOT NULL DEFAULT '';
ALTER TABLE marketplace_accounts ADD COLUMN notifications_seen_at INTEGER;
ALTER TABLE marketplace_accounts ADD COLUMN notifications_lost_at INTEGER;

CREATE UNIQUE INDEX marketplace_accounts_hook_text ON marketplace_accounts (hook_text) WHERE hook_text <> '';

-- A warehouse whose orders a courier takes one by one within hours (Яндекс Маркет's «Экспресс»).
ALTER TABLE marketplace_warehouses ADD COLUMN express INTEGER NOT NULL DEFAULT 0 CHECK (express IN (0, 1));

-- Every notification a marketplace sent, stored before it is answered: the account, when it arrived, its kind in the
-- marketplace's words, the order numbers it names (separated by spaces) and when they were read. No body is kept. Rows
-- older than 30 days are deleted by the nightly job of the journal.
CREATE TABLE marketplace_notifications (
    id         INTEGER PRIMARY KEY,
    account_id INTEGER NOT NULL REFERENCES marketplace_accounts (id) ON DELETE CASCADE,
    at         INTEGER NOT NULL,
    kind       TEXT    NOT NULL,
    numbers    TEXT    NOT NULL DEFAULT '',
    read_at    INTEGER
) STRICT;

CREATE INDEX marketplace_notifications_unread ON marketplace_notifications (account_id, read_at);
CREATE INDEX marketplace_notifications_at ON marketplace_notifications (at);
