-- The fourth migration (Plan 5): an order of several products, the deadline a marketplace gives, the order a marketplace
-- split another from, the warehouses whose orders the seller delivers itself, and the fields of a key that are shown.
-- Times are whole seconds since 1970-01-01 UTC, as everywhere.

-- The products of an order: one row per product, with how many units of it (at least one). An order of a marketplace
-- whose order is always one unit has one line of quantity 1. barcodes are separated by spaces. This is the one place a
-- product of an order is kept: the product columns of orders are moved here and dropped from it below.
CREATE TABLE order_lines (
    order_id   INTEGER NOT NULL REFERENCES orders (id) ON DELETE CASCADE,
    line       INTEGER NOT NULL CHECK (line >= 1),
    product_id TEXT    NOT NULL DEFAULT '',
    article    TEXT    NOT NULL DEFAULT '',
    name       TEXT    NOT NULL DEFAULT '',
    barcodes   TEXT    NOT NULL DEFAULT '',
    kind       TEXT    NOT NULL DEFAULT '',
    brand      TEXT    NOT NULL DEFAULT '',
    size       TEXT    NOT NULL DEFAULT '',
    quantity   INTEGER NOT NULL DEFAULT 1 CHECK (quantity >= 1),
    PRIMARY KEY (order_id, line)
) STRICT;

-- Every order kept so far is one unit of one product: it becomes one line of quantity 1 with its own product.
INSERT INTO order_lines (order_id, line, product_id, article, barcodes, kind, brand, size, quantity)
SELECT id, 1, product_id, article, barcodes, kind, brand, size, 1 FROM orders;

ALTER TABLE orders DROP COLUMN article;
ALTER TABLE orders DROP COLUMN barcodes;
ALTER TABLE orders DROP COLUMN product_id;
ALTER TABLE orders DROP COLUMN kind;
ALTER TABLE orders DROP COLUMN brand;
ALTER TABLE orders DROP COLUMN size;

-- The moment the marketplace says the order must be handed over by; NULL when it gives none, and the deadline is then
-- the creation time plus the account's hours, as before.
ALTER TABLE orders ADD COLUMN ship_by INTEGER;

-- The number of the order the marketplace split this one from; empty for an order that was not split off.
ALTER TABLE orders ADD COLUMN parent_number TEXT NOT NULL DEFAULT '';

CREATE INDEX orders_parent ON orders (account_id, parent_number);

-- A warehouse whose orders the seller delivers to the buyer itself: its orders need the buyer's address, which the wyo
-- server never keeps, so they are never read.
ALTER TABLE marketplace_warehouses ADD COLUMN seller_delivers INTEGER NOT NULL DEFAULT 0 CHECK (seller_delivers IN (0, 1));

-- The fields of the account's key that are not secret, shown in full (a number that tells two accounts of one marketplace
-- apart), as JSON [{"name","value"}]; empty for a key of one secret field. The key itself stays in the secret store.
ALTER TABLE marketplace_accounts ADD COLUMN key_fields TEXT NOT NULL DEFAULT '';
