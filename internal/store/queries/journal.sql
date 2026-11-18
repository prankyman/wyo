-- name: InsertJournalEntry :exec
INSERT INTO request_journal (
    at, account_id, marketplace, method, path, status, outcome, error_code, error_text, duration_ms, job
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListJournalEntries :many
SELECT j.*, COALESCE(a.name, '') AS account_name
FROM request_journal j
LEFT JOIN marketplace_accounts a ON a.id = j.account_id
WHERE (sqlc.narg(account_id) IS NULL OR j.account_id = sqlc.narg(account_id))
  AND (sqlc.narg(outcome) IS NULL OR j.outcome = sqlc.narg(outcome))
  AND (sqlc.narg(at_from) IS NULL OR j.at >= sqlc.narg(at_from))
  AND (sqlc.narg(at_before) IS NULL OR j.at < sqlc.narg(at_before))
ORDER BY j.at DESC, j.id DESC
LIMIT sqlc.arg(page_size) OFFSET sqlc.arg(page_offset);

-- name: CountJournalEntries :one
SELECT count(*) FROM request_journal j
WHERE (sqlc.narg(account_id) IS NULL OR j.account_id = sqlc.narg(account_id))
  AND (sqlc.narg(outcome) IS NULL OR j.outcome = sqlc.narg(outcome))
  AND (sqlc.narg(at_from) IS NULL OR j.at >= sqlc.narg(at_from))
  AND (sqlc.narg(at_before) IS NULL OR j.at < sqlc.narg(at_before));

-- name: DeleteJournalBefore :execrows
DELETE FROM request_journal WHERE at < ?;
