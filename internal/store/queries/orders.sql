-- name: InsertOrder :one
INSERT INTO orders (
    account_id, number, scheme, join_key, basket, warehouse_number, warehouse_name, article, barcodes, product_id, kind,
    brand, size, created_at, state, cancelled_by, seller_status, marketplace_status, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (account_id, number) DO NOTHING
RETURNING id;

-- name: GetOrderByNumber :one
SELECT * FROM orders WHERE account_id = ? AND number = ?;

-- name: GetOrderByJoinKey :one
SELECT * FROM orders WHERE account_id = ? AND join_key = ? ORDER BY id LIMIT 1;

-- name: UpdateOrderStatus :execrows
UPDATE orders SET state = ?, cancelled_by = ?, seller_status = ?, marketplace_status = ?, updated_at = ? WHERE id = ?;

-- name: UpdateOrderProduct :execrows
UPDATE orders SET basket = ?, kind = ?, brand = ?, size = ?, updated_at = ? WHERE id = ?;

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
  AND (sqlc.narg(search) IS NULL OR instr(o.number, sqlc.narg(search)) > 0 OR instr(o.article, sqlc.narg(search)) > 0
       OR instr(o.barcodes, sqlc.narg(search)) > 0)
ORDER BY o.created_at DESC, o.id DESC
LIMIT sqlc.arg(page_size) OFFSET sqlc.arg(page_offset);

-- name: CountOrderViews :one
SELECT count(*) FROM orders o
WHERE (sqlc.narg(account_id) IS NULL OR o.account_id = sqlc.narg(account_id))
  AND (sqlc.narg(scheme) IS NULL OR o.scheme = sqlc.narg(scheme))
  AND (sqlc.narg(state) IS NULL OR o.state = sqlc.narg(state))
  AND (sqlc.narg(created_from) IS NULL OR o.created_at >= sqlc.narg(created_from))
  AND (sqlc.narg(created_before) IS NULL OR o.created_at < sqlc.narg(created_before))
  AND (sqlc.narg(search) IS NULL OR instr(o.number, sqlc.narg(search)) > 0 OR instr(o.article, sqlc.narg(search)) > 0
       OR instr(o.barcodes, sqlc.narg(search)) > 0);

-- name: CountOrdersByState :many
SELECT o.state, count(*) AS orders FROM orders o
JOIN marketplace_accounts a ON a.id = o.account_id
WHERE a.state <> 'disabled'
GROUP BY o.state;

-- name: CountOverdueOrders :one
SELECT count(*) FROM orders o
JOIN marketplace_accounts a ON a.id = o.account_id
WHERE a.state <> 'disabled' AND o.scheme = 'fbs' AND o.state IN ('new', 'assembling')
  AND o.created_at + a.deadline_hours * 3600 < sqlc.arg(now);
