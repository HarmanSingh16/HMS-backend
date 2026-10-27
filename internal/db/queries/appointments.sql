-- name: GetActiveAppointmentsForDoctorInRange :many
SELECT id, doctor_id, patient_id, starts_at, ends_at, status
FROM appointments
WHERE doctor_id = $1
  AND status NOT IN ('cancelled')
  AND starts_at < $3
  AND ends_at > $2
ORDER BY starts_at ASC;

-- name: GetBookedAppointmentsForDoctorInRange :many
SELECT id, doctor_id, patient_id, starts_at, ends_at, status
FROM appointments
WHERE doctor_id = $1
  AND status = 'booked'
  AND starts_at < $3
  AND ends_at > $2
ORDER BY starts_at ASC;

-- name: CancelBookedAppointmentsInRange :many
UPDATE appointments
SET status = 'cancelled',
    cancellation_reason = $4,
    updated_at = NOW()
WHERE doctor_id = $1
  AND status = 'booked'
  AND starts_at < $3
  AND ends_at > $2
RETURNING id, doctor_id, patient_id, starts_at, ends_at, status, cancellation_reason;

-- name: CancelAppointmentByID :one
UPDATE appointments
SET status = 'cancelled',
    cancellation_reason = $2,
    updated_at = NOW()
WHERE id = $1
  AND status = 'booked'
RETURNING id, doctor_id, patient_id, starts_at, ends_at, status, cancellation_reason;

-- name: CreateAppointment :one
INSERT INTO appointments (
    doctor_id,
    patient_id,
    starts_at,
    ends_at,
    status
)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, doctor_id, patient_id, starts_at, ends_at, status, cancellation_reason, created_at, updated_at;

-- name: GetAppointmentByID :one
SELECT id, doctor_id, patient_id, starts_at, ends_at, status, cancellation_reason, created_at, updated_at
FROM appointments
WHERE id = $1;

-- name: GetAppointmentWithDetailsByID :one
SELECT 
    a.id,
    a.doctor_id,
    doc_acc.email AS doctor_email,
    d.department_id,
    dept.name AS department_name,
    a.patient_id,
    pat_acc.email AS patient_email,
    p.first_name AS patient_first_name,
    p.last_name AS patient_last_name,
    a.starts_at,
    a.ends_at,
    a.status,
    a.cancellation_reason,
    a.created_at,
    a.updated_at
FROM appointments a
JOIN accounts doc_acc ON a.doctor_id = doc_acc.id
JOIN doctors d ON a.doctor_id = d.account_id
LEFT JOIN departments dept ON d.department_id = dept.id
JOIN accounts pat_acc ON a.patient_id = pat_acc.id
JOIN patients p ON a.patient_id = p.account_id
WHERE a.id = $1;

-- name: ListAppointmentsWithDetails :many
SELECT 
    a.id,
    a.doctor_id,
    doc_acc.email AS doctor_email,
    d.department_id,
    dept.name AS department_name,
    a.patient_id,
    pat_acc.email AS patient_email,
    p.first_name AS patient_first_name,
    p.last_name AS patient_last_name,
    a.starts_at,
    a.ends_at,
    a.status,
    a.cancellation_reason,
    a.created_at,
    a.updated_at
FROM appointments a
JOIN accounts doc_acc ON a.doctor_id = doc_acc.id
JOIN doctors d ON a.doctor_id = d.account_id
LEFT JOIN departments dept ON d.department_id = dept.id
JOIN accounts pat_acc ON a.patient_id = pat_acc.id
JOIN patients p ON a.patient_id = p.account_id
WHERE 
    (sqlc.narg('doctor_id')::int IS NULL OR a.doctor_id = sqlc.narg('doctor_id'))
    AND (sqlc.narg('patient_id')::int IS NULL OR a.patient_id = sqlc.narg('patient_id'))
    AND (sqlc.narg('status')::appointment_status IS NULL OR a.status = sqlc.narg('status'))
    AND (sqlc.narg('starts_after')::timestamptz IS NULL OR a.starts_at >= sqlc.narg('starts_after'))
    AND (sqlc.narg('ends_before')::timestamptz IS NULL OR a.ends_at <= sqlc.narg('ends_before'))
ORDER BY a.starts_at ASC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountAppointmentsWithDetails :one
SELECT COUNT(*)
FROM appointments a
WHERE 
    (sqlc.narg('doctor_id')::int IS NULL OR a.doctor_id = sqlc.narg('doctor_id'))
    AND (sqlc.narg('patient_id')::int IS NULL OR a.patient_id = sqlc.narg('patient_id'))
    AND (sqlc.narg('status')::appointment_status IS NULL OR a.status = sqlc.narg('status'))
    AND (sqlc.narg('starts_after')::timestamptz IS NULL OR a.starts_at >= sqlc.narg('starts_after'))
    AND (sqlc.narg('ends_before')::timestamptz IS NULL OR a.ends_at <= sqlc.narg('ends_before'));

-- name: UpdateAppointmentStatus :one
UPDATE appointments
SET status = $2,
    cancellation_reason = $3,
    updated_at = NOW()
WHERE id = $1
RETURNING id, doctor_id, patient_id, starts_at, ends_at, status, cancellation_reason, created_at, updated_at;

-- name: CreateAppointmentStatusEvent :one
INSERT INTO appointment_status_events (
    appointment_id,
    from_status,
    to_status,
    changed_by,
    reason
)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, appointment_id, from_status, to_status, changed_by, reason, created_at;

-- name: ListAppointmentStatusEventsByAppointmentID :many
SELECT 
    e.id,
    e.appointment_id,
    e.from_status,
    e.to_status,
    e.changed_by,
    acc.email AS changed_by_email,
    e.reason,
    e.created_at
FROM appointment_status_events e
LEFT JOIN accounts acc ON e.changed_by = acc.id
WHERE e.appointment_id = $1
ORDER BY e.created_at ASC;

-- name: GetOverdueBookedAppointmentsForNoShow :many
SELECT id, doctor_id, patient_id, starts_at, ends_at, status
FROM appointments
WHERE status = 'booked'
  AND ends_at <= $1
ORDER BY ends_at ASC
LIMIT $2;