-- The fifth migration (Plan 6): an order's parcels, each with its own label, the reporting of the packing, and what a
-- warehouse tells about its labels. Times are whole seconds since 1970-01-01 UTC, as everywhere.

-- One closed box of an order at the fulfilment centre. An order has at least one parcel once it is packed; the parcels are
-- numbered from 1. mp_number is the marketplace's own number of the part the parcel became: the order's own number when the
-- marketplace does not split an order, the number of the part it made of the parcel when it does. The label of
-- a parcel is asked for later by some marketplaces: ticket is the marketplace's number of the asking, label_asked_at when it
-- was asked, label_next_at when it is read next, label_tries how many times; label_print is the fingerprint of the label
-- attached and label_sent_at when it was. A label that could not be made is not kept here: the order's problem holds it.
CREATE TABLE exchange_parcels (
    order_id       INTEGER NOT NULL REFERENCES orders (id) ON DELETE CASCADE,
    number         INTEGER NOT NULL CHECK (number >= 1),
    mp_number      TEXT    NOT NULL DEFAULT '',
    ticket         TEXT    NOT NULL DEFAULT '',
    label_print    TEXT    NOT NULL DEFAULT '',
    label_sent_at  INTEGER,
    label_asked_at INTEGER,
    label_next_at  INTEGER,
    label_tries    INTEGER NOT NULL DEFAULT 0 CHECK (label_tries >= 0),
    PRIMARY KEY (order_id, number)
) STRICT;

CREATE INDEX exchange_parcels_label_due ON exchange_parcels (label_next_at);

-- Every order sent so far has one label, which was kept in its own row: it becomes the order's first parcel, whose
-- marketplace number is the order's own number. label_next_at stays empty: a label already sent is never asked for again.
INSERT INTO exchange_parcels (order_id, number, mp_number, label_print, label_sent_at)
SELECT x.order_id, 1, o.number, x.label_print, x.label_sent_at
FROM exchange_orders x
JOIN orders o ON o.id = x.order_id;

ALTER TABLE exchange_orders DROP COLUMN label_print;
ALTER TABLE exchange_orders DROP COLUMN label_sent_at;

-- When the packing was reported to the marketplace and how many times, when the marketplace is read about it next, how many
-- times the marking codes were sent again because the marketplace did not manage to check them, and when the marketplace
-- first showed the order packed (its labels are asked a minute after).
ALTER TABLE exchange_orders ADD COLUMN pack_reported_at INTEGER;
ALTER TABLE exchange_orders ADD COLUMN pack_tries INTEGER NOT NULL DEFAULT 0 CHECK (pack_tries >= 0);
ALTER TABLE exchange_orders ADD COLUMN pack_next_at INTEGER;
ALTER TABLE exchange_orders ADD COLUMN codes_resends INTEGER NOT NULL DEFAULT 0 CHECK (codes_resends >= 0);
ALTER TABLE exchange_orders ADD COLUMN packed_at INTEGER;
-- The deadline last sent to the fulfilment centre (the marketplace's own when it gave one): a later change of the order's
-- deadline is passed on.
ALTER TABLE exchange_orders ADD COLUMN ship_by_sent INTEGER;

CREATE INDEX exchange_orders_pack_due ON exchange_orders (pack_next_at);

-- The order line a unit's code belongs to (the line's number in order_lines); 0 for a code kept before lines were known.
ALTER TABLE order_codes ADD COLUMN line INTEGER NOT NULL DEFAULT 0 CHECK (line >= 0);

-- The kind of label a warehouse's marketplace makes for it ('' when the marketplace names none) and whether the marketplace
-- itself assembles the warehouse's orders.
ALTER TABLE marketplace_warehouses ADD COLUMN label_type TEXT NOT NULL DEFAULT '';
ALTER TABLE marketplace_warehouses ADD COLUMN auto_assembly INTEGER NOT NULL DEFAULT 0 CHECK (auto_assembly IN (0, 1));
