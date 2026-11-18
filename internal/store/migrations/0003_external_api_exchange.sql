-- The third migration: the setting «Внешний API» and the exchange of orders with a fulfilment centre (Plan 4). Times are
-- whole seconds since 1970-01-01 UTC. No column holds a key (the keys are in the secret store), a buyer's datum, or a
-- marking code anywhere but order_codes, whose rows a nightly job deletes 90 days after their order was handed over.

-- The one row of «Внешний API»: the address of the fulfilment centre's API, the key's name in the secret store and its last
-- four characters, the state of the last check and what the fulfilment centre said of itself, how events are received, the
-- notifications' address text and the signing secret's name, and the number of the last event read in order.
CREATE TABLE fc_connection (
    id                   INTEGER PRIMARY KEY CHECK (id = 1),
    address              TEXT    NOT NULL,
    secret_name          TEXT    NOT NULL,
    key_last4            TEXT    NOT NULL,
    state                TEXT    NOT NULL CHECK (state IN ('working', 'key_refused', 'read_only_key', 'api_off', 'forbidden',
                                                           'unavailable', 'no_connection', 'unreadable')),
    seller_name          TEXT    NOT NULL DEFAULT '',
    -- The marketplaces the fulfilment centre accepts orders of, as JSON [{"code","name"}].
    marketplaces         TEXT    NOT NULL DEFAULT '[]',
    checked_at           INTEGER NOT NULL,
    events_mode          TEXT    NOT NULL DEFAULT 'poll' CHECK (events_mode IN ('poll', 'notify')),
    hook_text            TEXT    NOT NULL DEFAULT '',
    hook_secret_name     TEXT    NOT NULL DEFAULT '',
    events_mark          INTEGER NOT NULL DEFAULT 0 CHECK (events_mark >= 0),
    last_event_at        INTEGER,
    last_notification_at INTEGER,
    last_ping_at         INTEGER
) STRICT;

-- The fulfilment centre's warehouse that stands for a seller's warehouse on a marketplace. Only orders of a matched
-- warehouse are sent.
CREATE TABLE fc_warehouse_map (
    account_id       INTEGER NOT NULL REFERENCES marketplace_accounts (id) ON DELETE CASCADE,
    warehouse_number TEXT    NOT NULL,
    fc_warehouse     TEXT    NOT NULL,
    PRIMARY KEY (account_id, warehouse_number)
) STRICT;

-- The marketplace's code in the fulfilment centre's own list, for each account.
CREATE TABLE fc_marketplace_map (
    account_id     INTEGER PRIMARY KEY REFERENCES marketplace_accounts (id) ON DELETE CASCADE,
    fc_marketplace TEXT    NOT NULL
) STRICT;

-- Every event of the fulfilment centre, stored before it is applied: its id, its number in the fulfilment centre's
-- sequence, its kind and its body as received (an event carries no buyer's datum). An event already stored is ignored;
-- events are applied in the order of their numbers. Rows older than 30 days are deleted at night.
CREATE TABLE fc_events (
    id          TEXT    PRIMARY KEY,
    sequence    INTEGER NOT NULL UNIQUE CHECK (sequence > 0),
    type        TEXT    NOT NULL,
    body        TEXT    NOT NULL,
    received_at INTEGER NOT NULL,
    applied_at  INTEGER
) STRICT;

CREATE INDEX fc_events_unapplied ON fc_events (applied_at, sequence);

-- A marketplace's group of orders that leave together (some marketplaces call it a supply): the open one an account's orders of
-- one group are confirmed into, or the one of a hand-over.
CREATE TABLE marketplace_supplies (
    id          INTEGER PRIMARY KEY,
    account_id  INTEGER NOT NULL REFERENCES marketplace_accounts (id) ON DELETE RESTRICT,
    number      TEXT    NOT NULL,
    grp         TEXT    NOT NULL,
    role        TEXT    NOT NULL CHECK (role IN ('assembly', 'handover')),
    handover_id INTEGER REFERENCES handovers (id) ON DELETE RESTRICT,
    open        INTEGER NOT NULL DEFAULT 1 CHECK (open IN (0, 1)),
    deleted     INTEGER NOT NULL DEFAULT 0 CHECK (deleted IN (0, 1)),
    created_at  INTEGER NOT NULL,
    UNIQUE (account_id, number)
) STRICT;

CREATE INDEX marketplace_supplies_open ON marketplace_supplies (account_id, role, grp, open);

-- A hand-over the fulfilment centre asked documents for: its id and number there, its day, the warehouse and marketplace
-- codes it named, the transport boxes, the state of the steps, the step now, and the problem in the marketplace's words.
CREATE TABLE handovers (
    id                INTEGER PRIMARY KEY,
    fc_id             TEXT    NOT NULL UNIQUE,
    number            TEXT    NOT NULL,
    day               TEXT    NOT NULL,
    fc_warehouse      TEXT    NOT NULL DEFAULT '',
    fc_marketplace    TEXT    NOT NULL DEFAULT '',
    boxes             INTEGER NOT NULL DEFAULT 0 CHECK (boxes >= 0),
    state             TEXT    NOT NULL CHECK (state IN ('working', 'waiting', 'problem', 'documents_sent', 'withdrawn',
                                                        'completed')),
    step              TEXT    NOT NULL CHECK (step IN ('batch', 'move', 'boxes', 'shipping', 'close', 'documents')),
    closed            INTEGER NOT NULL DEFAULT 0 CHECK (closed IN (0, 1)),
    problem_code      TEXT    NOT NULL DEFAULT '',
    problem_params    TEXT    NOT NULL DEFAULT '{}',
    problem_mp_code   TEXT    NOT NULL DEFAULT '',
    problem_words     TEXT    NOT NULL DEFAULT '' CHECK (length(problem_words) <= 300),
    problem_at        INTEGER,
    problem_sent      INTEGER NOT NULL DEFAULT 0 CHECK (problem_sent IN (0, 1)),
    requested_at      INTEGER NOT NULL,
    updated_at        INTEGER NOT NULL
) STRICT;

CREATE INDEX handovers_requested ON handovers (requested_at, id);

-- The orders of a hand-over, as the fulfilment centre named them.
CREATE TABLE handover_orders (
    handover_id INTEGER NOT NULL REFERENCES handovers (id) ON DELETE CASCADE,
    order_id    INTEGER NOT NULL REFERENCES orders (id) ON DELETE CASCADE,
    PRIMARY KEY (handover_id, order_id)
) STRICT;

-- Each finished step of a hand-over, for each group of its orders: a wyo server stopped in the middle goes on from the
-- last finished step and never makes a write twice.
CREATE TABLE handover_steps (
    handover_id INTEGER NOT NULL REFERENCES handovers (id) ON DELETE CASCADE,
    account_id  INTEGER NOT NULL,
    grp         TEXT    NOT NULL,
    step        TEXT    NOT NULL CHECK (step IN ('batch', 'move', 'boxes', 'shipping', 'close', 'documents')),
    done_at     INTEGER NOT NULL,
    PRIMARY KEY (handover_id, account_id, grp, step)
) STRICT;

-- The exchange's own state of an order sent to the fulfilment centre: its id there (or the held order's id while it
-- waits), the state, the held reasons, the problem, the supply it stands in, the fingerprint of the label attached, the
-- verdicts' checking, the fulfilment centre's answer to a cancellation, and what needs the owner.
CREATE TABLE exchange_orders (
    order_id         INTEGER PRIMARY KEY REFERENCES orders (id) ON DELETE CASCADE,
    fc_id            TEXT    NOT NULL DEFAULT '',
    fc_number        TEXT    NOT NULL DEFAULT '',
    fc_held_id       TEXT    NOT NULL DEFAULT '',
    -- 'not_sent': the fulfilment centre refused the order; the problem says why.
    state            TEXT    NOT NULL CHECK (state IN ('not_sent', 'sent', 'held', 'reserved', 'label_sent', 'packed', 'codes_checking',
                                                       'codes_accepted', 'handed_over', 'cancelled')),
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
    label_print      TEXT    NOT NULL DEFAULT '',
    label_sent_at    INTEGER,
    codes_state      TEXT    NOT NULL DEFAULT '' CHECK (codes_state IN ('', 'to_send', 'checking', 'accepted', 'refused',
                                                                        'unfinished', 'not_taken', 'removing')),
    codes_sent_at    INTEGER,
    next_check_at    INTEGER,
    cancel_answer    TEXT    NOT NULL DEFAULT '' CHECK (cancel_answer IN ('', 'cancelled', 'waiting', 'needs_person',
                                                                          'handed_over')),
    handed_over_at   INTEGER,
    sent_at          INTEGER NOT NULL,
    updated_at       INTEGER NOT NULL
) STRICT;

CREATE INDEX exchange_orders_state ON exchange_orders (state);
CREATE INDEX exchange_orders_check ON exchange_orders (codes_state, next_check_at);
CREATE INDEX exchange_orders_fc_id ON exchange_orders (fc_id);

-- The exchange's history of an order: every change of its exchange state or a problem, with what happened.
CREATE TABLE exchange_history (
    id       INTEGER PRIMARY KEY,
    order_id INTEGER NOT NULL REFERENCES orders (id) ON DELETE CASCADE,
    at       INTEGER NOT NULL,
    state    TEXT    NOT NULL,
    code     TEXT    NOT NULL
) STRICT;

CREATE INDEX exchange_history_order ON exchange_history (order_id, at);

-- Each unit's identifier of an order, exactly as the fulfilment centre scanned it, with when it was sent to the
-- marketplace and the marketplace's verdict. Never shown in a log, a journal row or an answer of the wyo server's API.
CREATE TABLE order_codes (
    order_id   INTEGER NOT NULL REFERENCES orders (id) ON DELETE CASCADE,
    unit       INTEGER NOT NULL CHECK (unit > 0),
    identifier TEXT    NOT NULL,
    code       TEXT    NOT NULL,
    sent_at    INTEGER,
    verdict    TEXT    NOT NULL DEFAULT '',
    words      TEXT    NOT NULL DEFAULT '',
    PRIMARY KEY (order_id, unit)
) STRICT;

-- The group of orders that may travel together, the marketplace's own number of the goods' size, and the identifiers the
-- marketplace requires or accepts for the order (separated by spaces), as the list of new orders gave them.
ALTER TABLE orders ADD COLUMN grp TEXT NOT NULL DEFAULT '';
ALTER TABLE orders ADD COLUMN item_id TEXT NOT NULL DEFAULT '';
ALTER TABLE orders ADD COLUMN required_ids TEXT NOT NULL DEFAULT '';
ALTER TABLE orders ADD COLUMN optional_ids TEXT NOT NULL DEFAULT '';

-- The adapter's hand-over settings of an account (JSON {"name": "value"}) and the shipping point last chosen.
ALTER TABLE marketplace_accounts ADD COLUMN handover_settings TEXT NOT NULL DEFAULT '{}';
ALTER TABLE marketplace_accounts ADD COLUMN shipping_point TEXT NOT NULL DEFAULT '';
