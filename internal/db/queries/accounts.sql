-- name: GetAccountByEmail :one
SELECT id, email, password_hash, role, status, created_at
FROM accounts
WHERE email = $1 LIMIT 1;

-- name: GetAccountByID :one
SELECT id, email, password_hash, role, status, created_at
FROM accounts
WHERE id = $1 LIMIT 1;

-- name: CreateAccount :one
INSERT INTO accounts (email, password_hash, role, status)
VALUES ($1, $2, $3, $4)
RETURNING id, email, password_hash, role, status, created_at;

-- name: ActivateAccountWithPassword :exec
UPDATE accounts
SET password_hash = $2,
    status = 'active'
WHERE id = $1;

-- name: UpdateAccountStatus :exec
UPDATE accounts
SET status = $2
WHERE id = $1;

-- name: ListAccounts :many
SELECT 
    a.id,
    a.email,
    a.role,
    a.status,
    a.created_at,
    d.id AS department_id,
    d.name AS department_name
FROM accounts a
LEFT JOIN doctors doc ON doc.account_id = a.id
LEFT JOIN departments d ON d.id = doc.department_id
WHERE 
    ($1::user_role IS NULL OR a.role = $1)
    AND ($2::account_status IS NULL OR a.status = $2)
ORDER BY a.created_at DESC
LIMIT $3 OFFSET $4;

-- name: CountAccounts :one
SELECT COUNT(*)
FROM accounts a
WHERE 
    ($1::user_role IS NULL OR a.role = $1)
    AND ($2::account_status IS NULL OR a.status = $2);