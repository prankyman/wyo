-- The eighth migration (Plan 8, Яндекс Маркет's exchange): the label sizes the fulfilment centre names, the order's way out
-- and its «ready» write, the boxes the marketplace numbers, the kind of a batch the marketplace makes, and the checks of a
-- courier's code. Times are whole seconds since 1970-01-01 UTC, as everywhere. No column holds a key, a marking code, a
-- courier's code or a buyer's datum.

-- The label size the fulfilment centre names for one of its warehouses and one marketplace (its codes, as GET /warehouses of
-- the neutral API gives them), in millimetres, and when it was read. A warehouse and marketplace without a row has no size
-- named: the marketplace's own default is asked for.
CREATE TABLE fc_label_sizes (
    fc_warehouse   TEXT    NOT NULL,
    fc_marketplace TEXT    NOT NULL,
    width_mm       INTEGER NOT NULL CHECK (width_mm > 0),
    height_mm      INTEGER NOT NULL CHECK (height_mm > 0),
    read_at        INTEGER NOT NULL,
    PRIMARY KEY (fc_warehouse, fc_marketplace)
) STRICT;

-- When the fulfilment centre's label sizes were last read, whatever they named (none is an answer too).
ALTER TABLE fc_connection ADD COLUMN label_sizes_read_at INTEGER;

-- The exchange's own row of an order, made again because SQLite cannot change a CHECK: the state 'ready' (the marketplace
-- was told the order is ready to ship) joins the states; ready_at is when that was written or found; handover is how the order
-- leaves — 'shipment' with the others of its day, or 'courier_code' alone to a courier whose code was checked. Every row
-- kept before this migration leaves with a shipment and was never made ready.
CREATE TABLE exchange_orders_new (
    order_id         INTEGER PRIMARY KEY REFERENCES orders (id) ON DELETE CASCADE,
    fc_id            TEXT    NOT NULL DEFAULT '',
    fc_number        TEXT    NOT NULL DEFAULT '',
    fc_held_id       TEXT    NOT NULL DEFAULT '',
    -- 'not_sent': the fulfilment centre refused the order; the problem says why.
    state            TEXT    NOT NULL CHECK (state IN ('not_sent', 'sent', 'held', 'reserved', 'label_sent', 'packed', 'codes_checking',
                                                       'codes_accepted', 'ready', 'handed_over', 'cancelled')),
    -- The fulfilment centre's held reasons, as JSON [{"code","params"}].
    held_reasons     TEXT    NOT NULL DEFAULT '[]',
    attention        TEXT    NOT NULL DEFAULT '' CHECK (attention IN ('', 'held', 'problem', 'codes_refused',
                                                                      'cancelled_by_warehouse', 'change_waits')),
    problem_code     TEXT    NOT NULL DEFAULT '',
    problem_params   TEXT    NOT NULL DEFAULT '{}',
    problem_mp_code  TEXT    NOT NULL DEFAULT '',
    problem_words    TEXT    NOT NULL DEFAULT '' CHECK (length(problem_words) <= 300),
    problem_at       INTEGER,
    problem_sent     INTEGER NOT NULL DEFAULT 0 CHECK (problem_sent IN (0, 1)),
    -- The step to repeat when the owner presses «Передать заново» ('' when nothing failed).
    failed_step      TEXT    NOT NULL DEFAULT '',
    supply_id        INTEGER REFERENCES marketplace_supplies (id) ON DELETE SET NULL,
    confirmed_at     INTEGER,
    codes_state      TEXT    NOT NULL DEFAULT '' CHECK (codes_state IN ('', 'to_send', 'checking', 'accepted', 'refused',
                                                                        'unfinished', 'not_taken', 'removing')),
    codes_sent_at    INTEGER,
    next_check_at    INTEGER,
    cancel_answer    TEXT    NOT NULL DEFAULT '' CHECK (cancel_answer IN ('', 'cancelled', 'waiting', 'needs_person',
                                                                          'handed_over')),
    handed_over_at   INTEGER,
    sent_at          INTEGER NOT NULL,
    updated_at       INTEGER NOT NULL,
    pack_reported_at INTEGER,
    pack_tries       INTEGER NOT NULL DEFAULT 0 CHECK (pack_tries >= 0),
    pack_next_at     INTEGER,
    codes_resends    INTEGER NOT NULL DEFAULT 0 CHECK (codes_resends >= 0),
    packed_at        INTEGER,
    ship_by_sent     INTEGER,
    pack_sent_at     INTEGER,
    ready_at         INTEGER,
    handover         TEXT    NOT NULL DEFAULT 'shipment' CHECK (handover IN ('shipment', 'courier_code'))
) STRICT;

INSERT INTO exchange_orders_new (
    order_id, fc_id, fc_number, fc_held_id, state, held_reasons, attention, problem_code, problem_params, problem_mp_code,
    problem_words, problem_at, problem_sent, failed_step, supply_id, confirmed_at, codes_state, codes_sent_at, next_check_at,
    cancel_answer, handed_over_at, sent_at, updated_at, pack_reported_at, pack_tries, pack_next_at, codes_resends, packed_at,
    ship_by_sent, pack_sent_at
)
SELECT
    order_id, fc_id, fc_number, fc_held_id, state, held_reasons, attention, problem_code, problem_params, problem_mp_code,
    problem_words, problem_at, problem_sent, failed_step, supply_id, confirmed_at, codes_state, codes_sent_at, next_check_at,
    cancel_answer, handed_over_at, sent_at, updated_at, pack_reported_at, pack_tries, pack_next_at, codes_resends, packed_at,
    ship_by_sent, pack_sent_at
FROM exchange_orders;

DROP TABLE exchange_orders;

ALTER TABLE exchange_orders_new RENAME TO exchange_orders;

CREATE INDEX exchange_orders_state ON exchange_orders (state);
CREATE INDEX exchange_orders_check ON exchange_orders (codes_state, next_check_at);
CREATE INDEX exchange_orders_fc_id ON exchange_orders (fc_id);
CREATE INDEX exchange_orders_pack_due ON exchange_orders (pack_next_at);

-- The marketplace's own number of the box a parcel became, for a marketplace that numbers the boxes of an order ('' for one
-- that does not).
ALTER TABLE exchange_parcels ADD COLUMN box TEXT NOT NULL DEFAULT '';

-- The kind of a batch the marketplace makes by itself, in the marketplace's words ('' for a batch the wyo server made).
ALTER TABLE marketplace_supplies ADD COLUMN kind TEXT NOT NULL DEFAULT '';

-- One code a courier named for an order that leaves alone, as the fulfilment centre reported it: the fulfilment centre's number
-- of the check, when it came, the marketplace's answer ('' until it is given: 'accepted', 'refused', 'new_code_needed'), the
-- attempts left when the marketplace names them, its own words, and when the answer was sent back. The code itself is never
-- kept.
CREATE TABLE exchange_courier_checks (
    check_id      TEXT    PRIMARY KEY,
    order_id      INTEGER NOT NULL REFERENCES orders (id) ON DELETE CASCADE,
    came_at       INTEGER NOT NULL,
    result        TEXT    NOT NULL DEFAULT '' CHECK (result IN ('', 'accepted', 'refused', 'new_code_needed')),
    attempts_left INTEGER CHECK (attempts_left IS NULL OR attempts_left >= 0),
    words         TEXT    NOT NULL DEFAULT '' CHECK (length(words) <= 300),
    answered_at   INTEGER
) STRICT;

CREATE INDEX exchange_courier_checks_order ON exchange_courier_checks (order_id, came_at);
