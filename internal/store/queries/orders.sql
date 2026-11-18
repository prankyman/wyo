-- name: InsertOrder :one
INSERT INTO orders (
    account_id, number, scheme, join_key, basket, warehouse_number, warehouse_name, created_at, ship_by, parent_number, state,
    cancelled_by, seller_status, marketplace_status, updated_at, grp, item_id, required_ids, optional_ids
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (account_id, number) DO NOTHING
RETURNING id;

-- name: InsertOrderLine :exec
INSERT INTO order_lines (order_id, line, product_id, article, name, barcodes, kind, brand, size, quantity)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: DeleteOrderLines :exec
DELETE FROM order_lines WHERE order_id = ?;

-- name: ListOrderLines :many
SELECT * FROM order_lines WHERE order_id = ? ORDER BY line;

-- name: ListOrderLinesOf :many
SELECT * FROM order_lines WHERE order_id IN (sqlc.slice(order_ids)) ORDER BY order_id, line;

-- name: UpdateOrderShipBy :execrows
UPDATE orders SET ship_by = ?, updated_at = ? WHERE id = ?;

-- name: ListOrderParts :many
SELECT number FROM orders WHERE account_id = ? AND parent_number = ? AND parent_number <> '' ORDER BY number;

-- name: ListOrdersOfNumbers :many
SELECT * FROM orders WHERE account_id = ? AND number IN (sqlc.slice(numbers));

-- name: GetOrderByNumber :one
SELECT * FROM orders WHERE account_id = ? AND number = ?;

-- name: GetOrderByJoinKey :one
SELECT * FROM orders WHERE account_id = ? AND join_key = ? ORDER BY id LIMIT 1;

-- name: UpdateOrderStatus :execrows
UPDATE orders SET state = ?, cancelled_by = ?, seller_status = ?, marketplace_status = ?, updated_at = ? WHERE id = ?;

-- name: UpdateOrderBasket :execrows
UPDATE orders SET basket = ?, updated_at = ? WHERE id = ?;

-- name: InsertOrderEvent :exec
INSERT INTO order_events (order_id, at, state, cancelled_by, seller_status, marketplace_status, source)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: ListOrderEvents :many
SELECT * FROM order_events WHERE order_id = ? ORDER BY at, id;

-- name: ListOpenOrderNumbers :many
SELECT number FROM orders
WHERE account_id = ? AND scheme = ? AND state NOT IN ('received', 'cancelled') AND number > ?
ORDER BY number
LIMIT ?;

-- name: GetOrderView :one
SELECT o.*, a.name AS account_name, a.marketplace AS marketplace, a.deadline_hours AS deadline_hours,
       COALESCE(w.name, '') AS known_warehouse_name
FROM orders o
JOIN marketplace_accounts a ON a.id = o.account_id
LEFT JOIN marketplace_warehouses w ON w.account_id = o.account_id AND w.number = o.warehouse_number
WHERE o.id = ?;

-- name: ListOrderViews :many
SELECT o.*, a.name AS account_name, a.marketplace AS marketplace, a.deadline_hours AS deadline_hours,
       COALESCE(w.name, '') AS known_warehouse_name
FROM orders o
JOIN marketplace_accounts a ON a.id = o.account_id
LEFT JOIN marketplace_warehouses w ON w.account_id = o.account_id AND w.number = o.warehouse_number
WHERE (sqlc.narg(account_id) IS NULL OR o.account_id = sqlc.narg(account_id))
  AND (sqlc.narg(scheme) IS NULL OR o.scheme = sqlc.narg(scheme))
  AND (sqlc.narg(state) IS NULL OR o.state = sqlc.narg(state))
  AND (sqlc.narg(created_from) IS NULL OR o.created_at >= sqlc.narg(created_from))
  AND (sqlc.narg(created_before) IS NULL OR o.created_at < sqlc.narg(created_before))
  AND (sqlc.narg(search) IS NULL OR instr(o.number, sqlc.narg(search)) > 0 OR EXISTS (
       SELECT 1 FROM order_lines l
       WHERE l.order_id = o.id
         AND (instr(l.article, sqlc.narg(search)) > 0 OR instr(l.barcodes, sqlc.narg(search)) > 0
              OR instr(l.product_id, sqlc.narg(search)) > 0)))
  AND (sqlc.narg(attention_now) IS NULL OR EXISTS (
       SELECT 1 FROM exchange_orders x
       WHERE x.order_id = o.id
         AND (x.attention <> '' OR (x.state = 'sent' AND o.created_at + a.deadline_hours * 900 <= sqlc.narg(attention_now)))))
ORDER BY o.created_at DESC, o.id DESC
LIMIT sqlc.arg(page_size) OFFSET sqlc.arg(page_offset);

-- name: CountOrderViews :one
SELECT count(*) FROM orders o
JOIN marketplace_accounts a ON a.id = o.account_id
WHERE (sqlc.narg(account_id) IS NULL OR o.account_id = sqlc.narg(account_id))
  AND (sqlc.narg(scheme) IS NULL OR o.scheme = sqlc.narg(scheme))
  AND (sqlc.narg(state) IS NULL OR o.state = sqlc.narg(state))
  AND (sqlc.narg(created_from) IS NULL OR o.created_at >= sqlc.narg(created_from))
  AND (sqlc.narg(created_before) IS NULL OR o.created_at < sqlc.narg(created_before))
  AND (sqlc.narg(search) IS NULL OR instr(o.number, sqlc.narg(search)) > 0 OR EXISTS (
       SELECT 1 FROM order_lines l
       WHERE l.order_id = o.id
         AND (instr(l.article, sqlc.narg(search)) > 0 OR instr(l.barcodes, sqlc.narg(search)) > 0
              OR instr(l.product_id, sqlc.narg(search)) > 0)))
  AND (sqlc.narg(attention_now) IS NULL OR EXISTS (
       SELECT 1 FROM exchange_orders x
       WHERE x.order_id = o.id
         AND (x.attention <> '' OR (x.state = 'sent' AND o.created_at + a.deadline_hours * 900 <= sqlc.narg(attention_now)))));

-- name: CountOrdersByState :many
SELECT o.state, count(*) AS orders FROM orders o
JOIN marketplace_accounts a ON a.id = o.account_id
WHERE a.state <> 'disabled'
GROUP BY o.state;

-- name: CountOverdueOrders :one
SELECT count(*) FROM orders o
JOIN marketplace_accounts a ON a.id = o.account_id
WHERE a.state <> 'disabled' AND o.scheme = 'fbs' AND o.state IN ('new', 'assembling')
  AND COALESCE(o.ship_by, o.created_at + a.deadline_hours * 3600) < CAST(sqlc.arg(now) AS INTEGER);
