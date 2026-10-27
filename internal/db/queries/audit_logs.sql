-- name: CreateAuditLog :one
INSERT INTO audit_logs (
    actor_id,
    action,
    entity_type,
    entity_id,
    details
)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, actor_id, action, entity_type, entity_id, details, created_at;

-- name: ListAuditLogs :many
SELECT 
    al.id,
    al.actor_id,
    a.email AS actor_email,
    al.action,
    al.entity_type,
    al.entity_id,
    al.details,
    al.created_at
FROM audit_logs al
LEFT JOIN accounts a ON a.id = al.actor_id
WHERE 
    ($1::int IS NULL OR al.actor_id = $1)
    AND ($2::text IS NULL OR al.action = $2)
    AND ($3::text IS NULL OR al.entity_type = $3)
ORDER BY al.created_at DESC
LIMIT $4 OFFSET $5;

SELECT COUNT(*)
FROM audit_logs al
WHERE 
    ($1::int IS NULL OR al.actor_id = $1)
    AND ($2::text IS NULL OR al.action = $2)
    AND ($3::text IS NULL OR al.entity_type = $3);