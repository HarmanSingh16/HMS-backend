-- name: CreateInvite :one
INSERT INTO invites (account_id, token_hash, expires_at)
VALUES ($1, $2, $3)
RETURNING id, account_id, token_hash, expires_at, claimed_at, created_at;

-- name: GetInviteByTokenHash :one
-- Joins accounts so we can return the associated email and role in a single DB query
SELECT 
    i.id AS invite_id,
    i.account_id,
    i.token_hash,
    i.expires_at,
    i.claimed_at,
    a.email,
    a.role,
    a.status AS account_status
FROM invites i
JOIN accounts a ON a.id = i.account_id
WHERE i.token_hash = $1
LIMIT 1;

-- name: MarkInviteClaimed :exec
UPDATE invites
SET claimed_at = NOW()
WHERE id = $1;