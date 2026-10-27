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

-- name: CancelPendingNotificationsByAppointmentID :exec
UPDATE notifications
SET status = 'cancelled'
WHERE appointment_id = $1
  AND status = 'pending';

-- name: GetPendingNotificationsForDispatch :many
SELECT id, recipient_id, invite_id, appointment_id, type, status, payload, scheduled_for, retry_count
FROM notifications
WHERE status = 'pending'
  AND scheduled_for <= $1
ORDER BY scheduled_for ASC
LIMIT $2
FOR UPDATE SKIP LOCKED;

-- name: UpdateNotificationStatus :exec
UPDATE notifications
SET status = $2,
    sent_at = $3,
    retry_count = retry_count + 1,
    last_error = $4
WHERE id = $1;