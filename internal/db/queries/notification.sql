-- name: CreateNotification :one
INSERT INTO notifications (
    recipient_id,
    invite_id,
    appointment_id,
    type,
    status,
    payload,
    scheduled_for
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, recipient_id, invite_id, appointment_id, type, status, payload, scheduled_for, sent_at, retry_count, last_error, created_at;