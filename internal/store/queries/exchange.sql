-- name: GetConnection :one
SELECT * FROM fc_connection WHERE id = 1;

-- name: PutConnection :exec
INSERT INTO fc_connection (id, address, secret_name, key_last4, state, seller_name, marketplaces, checked_at)
VALUES (1, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (id) DO UPDATE SET address = excluded.address, secret_name = excluded.secret_name,
    key_last4 = excluded.key_last4, state = excluded.state, seller_name = excluded.seller_name,
    marketplaces = excluded.marketplaces, checked_at = excluded.checked_at;

-- name: SetConnectionState :execrows
UPDATE fc_connection SET state = ?, checked_at = ? WHERE id = 1;

-- name: SetConnectionFacts :execrows
UPDATE fc_connection SET state = ?, seller_name = ?, marketplaces = ?, checked_at = ? WHERE id = 1;

-- name: DeleteConnection :execrows
DELETE FROM fc_connection WHERE id = 1;

-- name: SetEventsMode :execrows
UPDATE fc_connection SET events_mode = ?, hook_text = ?, hook_secret_name = ? WHERE id = 1;

-- name: SetEventsMark :execrows
UPDATE fc_connection SET events_mark = ?, last_event_at = COALESCE(sqlc.narg(last_event_at), last_event_at) WHERE id = 1;

-- name: TouchNotification :execrows
UPDATE fc_connection SET last_notification_at = sqlc.arg(at),
    last_ping_at = CASE WHEN sqlc.arg(ping) = 1 THEN sqlc.arg(at) ELSE last_ping_at END
WHERE id = 1;

-- name: ListWarehouseMatches :many
SELECT * FROM fc_warehouse_map ORDER BY account_id, warehouse_number;

-- name: PutWarehouseMatch :exec
INSERT INTO fc_warehouse_map (account_id, warehouse_number, fc_warehouse) VALUES (?, ?, ?)
ON CONFLICT (account_id, warehouse_number) DO UPDATE SET fc_warehouse = excluded.fc_warehouse;

-- name: DeleteWarehouseMatch :exec
DELETE FROM fc_warehouse_map WHERE account_id = ? AND warehouse_number = ?;

-- name: ListMarketplaceMatches :many
SELECT * FROM fc_marketplace_map ORDER BY account_id;

-- name: PutMarketplaceMatch :exec
INSERT INTO fc_marketplace_map (account_id, fc_marketplace) VALUES (?, ?)
ON CONFLICT (account_id) DO UPDATE SET fc_marketplace = excluded.fc_marketplace;

-- name: DeleteMarketplaceMatch :exec
DELETE FROM fc_marketplace_map WHERE account_id = ?;

-- name: SetAccountHandoverSettings :execrows
UPDATE marketplace_accounts SET handover_settings = ? WHERE id = ?;

-- name: SetAccountShippingPoint :execrows
UPDATE marketplace_accounts SET shipping_point = ? WHERE id = ?;

-- name: GetAccountHandover :one
SELECT handover_settings, shipping_point FROM marketplace_accounts WHERE id = ?;

-- name: InsertFcEvent :execrows
INSERT INTO fc_events (id, sequence, type, body, received_at) VALUES (?, ?, ?, ?, ?)
ON CONFLICT DO NOTHING;

-- name: ListUnappliedFcEvents :many
SELECT * FROM fc_events WHERE applied_at IS NULL AND sequence <= ? ORDER BY sequence LIMIT ?;

-- name: MarkFcEventApplied :exec
UPDATE fc_events SET applied_at = ? WHERE id = ?;

-- name: DeleteFcEventsBefore :execrows
DELETE FROM fc_events WHERE received_at < ? AND applied_at IS NOT NULL;

-- name: OrdersToSend :many
-- Only the orders of the marketplaces whose adapters hand orders over (the list marketplaces) are sent, and none whose own
-- status at the marketplace is one its adapter says waits (a JSON list of texts, each a marketplace and a status). The limit is a
-- plain placeholder: sqlc numbers a named one after the first placeholder of a slice (`?2`), which with two marketplaces is
-- the name of the second marketplace, not the limit.
SELECT o.id, o.account_id, o.number, o.basket, o.item_id, o.created_at, o.ship_by, o.warehouse_number, o.test, o.scheme_name,
       a.deadline_hours, m.fc_marketplace, w.fc_warehouse
FROM orders o
JOIN marketplace_accounts a ON a.id = o.account_id
JOIN fc_marketplace_map m ON m.account_id = o.account_id
JOIN fc_warehouse_map w ON w.account_id = o.account_id AND w.warehouse_number = o.warehouse_number
LEFT JOIN exchange_orders x ON x.order_id = o.id
WHERE o.scheme = 'fbs' AND o.state = 'new' AND x.order_id IS NULL
  AND a.state IN ('working', 'key_expiring', 'no_connection') AND a.read_only = 0
  AND a.marketplace IN (sqlc.slice(marketplaces))
  AND NOT EXISTS (SELECT 1 FROM json_each(CAST(? AS TEXT)) j WHERE j.value = a.marketplace || char(32) || o.marketplace_status)
ORDER BY o.created_at, o.id
LIMIT ?;

-- name: GetExchangeOrder :one
SELECT * FROM exchange_orders WHERE order_id = ?;

-- name: GetExchangeOrderByFcID :one
SELECT * FROM exchange_orders WHERE fc_id = ? AND fc_id <> '' LIMIT 1;

-- name: GetExchangeOrderByHeldID :one
SELECT * FROM exchange_orders WHERE fc_held_id = ? AND fc_held_id <> '' LIMIT 1;

-- name: SaveExchangeOrder :exec
INSERT INTO exchange_orders (
    order_id, fc_id, fc_number, fc_held_id, state, held_reasons, attention, problem_code, problem_params, problem_mp_code,
    problem_words, problem_at, problem_sent, failed_step, supply_id, confirmed_at, pack_reported_at, pack_tries, pack_next_at,
    codes_resends, packed_at, ship_by_sent, codes_state, codes_sent_at, next_check_at, cancel_answer, handed_over_at, sent_at,
    updated_at, pack_sent_at, ready_at, handover
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (order_id) DO UPDATE SET fc_id = excluded.fc_id, fc_number = excluded.fc_number,
    fc_held_id = excluded.fc_held_id, state = excluded.state, held_reasons = excluded.held_reasons,
    attention = excluded.attention, problem_code = excluded.problem_code, problem_params = excluded.problem_params,
    problem_mp_code = excluded.problem_mp_code, problem_words = excluded.problem_words, problem_at = excluded.problem_at,
    problem_sent = excluded.problem_sent, failed_step = excluded.failed_step, supply_id = excluded.supply_id,
    confirmed_at = excluded.confirmed_at, pack_reported_at = excluded.pack_reported_at, pack_tries = excluded.pack_tries,
    pack_next_at = excluded.pack_next_at, codes_resends = excluded.codes_resends, packed_at = excluded.packed_at, ship_by_sent = excluded.ship_by_sent,
    codes_state = excluded.codes_state, codes_sent_at = excluded.codes_sent_at, next_check_at = excluded.next_check_at,
    cancel_answer = excluded.cancel_answer, handed_over_at = excluded.handed_over_at, updated_at = excluded.updated_at,
    pack_sent_at = excluded.pack_sent_at, ready_at = excluded.ready_at, handover = excluded.handover;

-- name: GetExchangeOrderByNumber :one
SELECT x.* FROM exchange_orders x
JOIN orders o ON o.id = x.order_id
JOIN fc_marketplace_map m ON m.account_id = o.account_id
WHERE m.fc_marketplace = ? AND o.number = ?
ORDER BY o.id LIMIT 1;

-- name: ListExchangeOrdersOf :many
SELECT * FROM exchange_orders WHERE order_id IN (sqlc.slice(ids));

-- name: ListOpenExchangeOrders :many
SELECT x.* FROM exchange_orders x
JOIN orders o ON o.id = x.order_id
WHERE x.state NOT IN ('handed_over', 'cancelled') AND x.cancel_answer = ''
  AND (sqlc.narg(order_state) IS NULL OR o.state = sqlc.narg(order_state))
ORDER BY x.order_id;

-- name: ListDeadlineChangedExchangeOrders :many
SELECT x.* FROM exchange_orders x
JOIN orders o ON o.id = x.order_id
WHERE x.state NOT IN ('handed_over', 'cancelled') AND x.cancel_answer = '' AND x.fc_id <> ''
  AND o.ship_by IS NOT NULL AND (x.ship_by_sent IS NULL OR x.ship_by_sent <> o.ship_by)
ORDER BY x.order_id;

-- name: ListExchangeOrdersByStep :many
SELECT * FROM exchange_orders WHERE failed_step <> '' ORDER BY order_id;

-- name: ListDueVerdictChecks :many
SELECT * FROM exchange_orders WHERE codes_state = 'checking' AND next_check_at <= ? ORDER BY next_check_at LIMIT ?;

-- name: ListDuePackChecks :many
SELECT * FROM exchange_orders
WHERE pack_next_at <= ? AND state NOT IN ('handed_over', 'cancelled') AND cancel_answer = ''
ORDER BY pack_next_at, order_id LIMIT ?;

-- name: CountExchangeAttention :many
SELECT x.attention AS code, count(*) AS orders FROM exchange_orders x
WHERE x.attention <> ''
GROUP BY x.attention;

-- name: CountWarehouseUnknown :one
-- The new orders of a matched marketplace that are not sent because the marketplace did not tell their warehouse.
SELECT count(*) FROM orders o
WHERE o.scheme = 'fbs' AND o.state = 'new' AND o.warehouse_number = '' AND o.warehouse_text <> ''
  AND EXISTS (SELECT 1 FROM fc_connection) AND EXISTS (SELECT 1 FROM fc_marketplace_map m WHERE m.account_id = o.account_id);

-- name: CountNotReservedInTime :one
SELECT count(*) FROM exchange_orders x
JOIN orders o ON o.id = x.order_id
JOIN marketplace_accounts a ON a.id = o.account_id
WHERE x.state = 'sent' AND x.attention = '' AND o.created_at + a.deadline_hours * 900 <= ?;

-- name: InsertExchangeHistory :exec
INSERT INTO exchange_history (order_id, at, state, code) VALUES (?, ?, ?, ?);

-- name: ListExchangeHistory :many
SELECT * FROM exchange_history WHERE order_id = ? ORDER BY at, id;

-- name: DeleteOrderCodes :exec
DELETE FROM order_codes WHERE order_id = ?;

-- name: InsertOrderCode :exec
INSERT INTO order_codes (order_id, unit, line, identifier, code) VALUES (?, ?, ?, ?, ?);

-- name: ListOrderCodes :many
SELECT * FROM order_codes WHERE order_id = ? ORDER BY unit;

-- name: MarkOrderCodesSent :exec
UPDATE order_codes SET sent_at = ? WHERE order_id = ? AND identifier = ?;

-- name: SetOrderCodesVerdict :exec
UPDATE order_codes SET verdict = ?, words = ? WHERE order_id = ? AND identifier = ?;

-- name: DeleteCodesHandedOverBefore :execrows
DELETE FROM order_codes WHERE order_id IN (
    SELECT order_id FROM exchange_orders WHERE handed_over_at IS NOT NULL AND handed_over_at < ?
);

-- name: GetOpenAssemblySupply :one
SELECT * FROM marketplace_supplies
WHERE account_id = ? AND role = 'assembly' AND grp = ? AND open = 1 AND deleted = 0
ORDER BY id DESC LIMIT 1;

-- name: InsertSupply :one
INSERT INTO marketplace_supplies (account_id, number, grp, role, handover_id, created_at, kind) VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: GetSupply :one
SELECT * FROM marketplace_supplies WHERE id = ?;

-- name: SetSupplyOpen :exec
UPDATE marketplace_supplies SET open = ? WHERE id = ?;

-- name: SetSupplyDeleted :exec
UPDATE marketplace_supplies SET deleted = 1, open = 0 WHERE id = ?;

-- name: ListHandoverSupplies :many
SELECT * FROM marketplace_supplies WHERE handover_id = ? AND deleted = 0 ORDER BY id;

-- name: InsertHandover :one
INSERT INTO handovers (fc_id, number, day, fc_warehouse, fc_marketplace, boxes, state, step, requested_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, 'working', 'batch', ?, ?)
ON CONFLICT (fc_id) DO UPDATE SET boxes = excluded.boxes, updated_at = excluded.updated_at,
    state = CASE WHEN handovers.state IN ('withdrawn') THEN 'working' ELSE handovers.state END
RETURNING id;

-- name: GetHandover :one
SELECT * FROM handovers WHERE id = ?;

-- name: GetHandoverByFcID :one
SELECT * FROM handovers WHERE fc_id = ?;

-- name: SaveHandover :exec
UPDATE handovers SET state = ?, step = ?, closed = ?, problem_code = ?, problem_params = ?, problem_mp_code = ?,
    problem_words = ?, problem_at = ?, problem_sent = ?, updated_at = ?
WHERE id = ?;

-- name: ListHandovers :many
SELECT * FROM handovers ORDER BY requested_at DESC, id DESC LIMIT ?;

-- name: ListWorkingHandovers :many
SELECT * FROM handovers WHERE state IN ('working', 'waiting') ORDER BY id;

-- name: CountHandoversWaiting :one
SELECT count(*) FROM handovers WHERE state IN ('waiting', 'problem');

-- name: InsertHandoverOrder :exec
INSERT INTO handover_orders (handover_id, order_id) VALUES (?, ?) ON CONFLICT DO NOTHING;

-- name: DeleteHandoverOrders :exec
DELETE FROM handover_orders WHERE handover_id = ?;

-- name: ListHandoverOrders :many
SELECT o.id, o.account_id, o.number, o.grp, o.warehouse_number, COALESCE(x.supply_id, 0) AS supply_id
FROM handover_orders h
JOIN orders o ON o.id = h.order_id
LEFT JOIN exchange_orders x ON x.order_id = o.id
WHERE h.handover_id = ?
ORDER BY o.account_id, o.grp, o.id;

-- name: InsertHandoverStep :exec
INSERT INTO handover_steps (handover_id, account_id, grp, step, done_at) VALUES (?, ?, ?, ?, ?) ON CONFLICT DO NOTHING;

-- name: ListHandoverSteps :many
SELECT * FROM handover_steps WHERE handover_id = ? ORDER BY done_at;

-- name: DeleteHandoverSteps :exec
DELETE FROM handover_steps WHERE handover_id = ?;

-- name: DeleteExchangeOrder :exec
DELETE FROM exchange_orders WHERE order_id = ?;

-- name: SaveParcel :exec
INSERT INTO exchange_parcels (order_id, number, mp_number, ticket, label_print, label_sent_at, label_asked_at, label_next_at, label_tries, box)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (order_id, number) DO UPDATE SET mp_number = excluded.mp_number, ticket = excluded.ticket, box = excluded.box,
    label_print = excluded.label_print, label_sent_at = excluded.label_sent_at, label_asked_at = excluded.label_asked_at,
    label_next_at = excluded.label_next_at, label_tries = excluded.label_tries;

-- name: DeleteParcelsOf :exec
DELETE FROM exchange_parcels WHERE order_id = ?;

-- name: ListParcelsOf :many
SELECT * FROM exchange_parcels WHERE order_id IN (sqlc.slice(ids)) ORDER BY order_id, number;

-- name: ListDueLabelChecks :many
SELECT p.* FROM exchange_parcels p
JOIN exchange_orders x ON x.order_id = p.order_id
WHERE p.label_next_at <= ? AND p.label_sent_at IS NULL
  AND x.state NOT IN ('handed_over', 'cancelled') AND x.cancel_answer = ''
ORDER BY p.label_next_at, p.order_id, p.number LIMIT ?;

-- name: SetSupplyKind :exec
UPDATE marketplace_supplies SET kind = ? WHERE id = ?;

-- name: DeleteLabelSizes :exec
DELETE FROM fc_label_sizes;

-- name: InsertLabelSize :exec
INSERT INTO fc_label_sizes (fc_warehouse, fc_marketplace, width_mm, height_mm, read_at) VALUES (?, ?, ?, ?, ?)
ON CONFLICT (fc_warehouse, fc_marketplace) DO UPDATE SET width_mm = excluded.width_mm, height_mm = excluded.height_mm,
    read_at = excluded.read_at;

-- name: SetLabelSizesRead :execrows
UPDATE fc_connection SET label_sizes_read_at = ? WHERE id = 1;

-- name: ListLabelSizes :many
SELECT * FROM fc_label_sizes ORDER BY fc_warehouse, fc_marketplace;

-- name: GetOrderFcPlace :one
-- The fulfilment centre's warehouse and marketplace an order is matched to.
SELECT w.fc_warehouse, m.fc_marketplace
FROM orders o
JOIN fc_marketplace_map m ON m.account_id = o.account_id
JOIN fc_warehouse_map w ON w.account_id = o.account_id AND w.warehouse_number = o.warehouse_number
WHERE o.id = ?;

-- name: InsertCourierCheck :execrows
INSERT INTO exchange_courier_checks (check_id, order_id, came_at) VALUES (?, ?, ?) ON CONFLICT (check_id) DO NOTHING;

-- name: AnswerCourierCheck :exec
UPDATE exchange_courier_checks SET result = ?, attempts_left = ?, words = ?, answered_at = ? WHERE check_id = ?;

-- name: GetCourierCheck :one
SELECT * FROM exchange_courier_checks WHERE check_id = ?;

-- name: ListCourierChecks :many
SELECT * FROM exchange_courier_checks WHERE order_id IN (sqlc.slice(ids)) ORDER BY order_id, came_at, check_id;

-- name: SetFcEventBody :exec
UPDATE fc_events SET body = ? WHERE id = ?;

-- name: ListTestOrderNumbers :many
-- The numbers among the given ones of the test orders of an account.
SELECT number FROM orders WHERE account_id = ? AND test = 1 AND number IN (sqlc.slice(numbers)) ORDER BY number;
