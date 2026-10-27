-- name: GetDoctorOfficeHours :many
SELECT id, doctor_id, weekday, start_time, end_time, created_at, updated_at
FROM doctor_office_hours
WHERE doctor_id = $1
ORDER BY weekday ASC, start_time ASC;

-- name: DeleteDoctorOfficeHours :exec
DELETE FROM doctor_office_hours
WHERE doctor_id = $1;

-- name: CreateDoctorOfficeHour :one
INSERT INTO doctor_office_hours (
    doctor_id,
    weekday,
    start_time,
    end_time
)
VALUES ($1, $2, $3, $4)
RETURNING id, doctor_id, weekday, start_time, end_time, created_at, updated_at;

-- name: CreateDoctorTimeOff :one
INSERT INTO doctor_time_off (
    doctor_id,
    starts_at,
    ends_at,
    reason
)
VALUES ($1, $2, $3, $4)
RETURNING id, doctor_id, starts_at, ends_at, reason, created_at;

-- name: ListUpcomingDoctorTimeOff :many
SELECT id, doctor_id, starts_at, ends_at, reason, created_at
FROM doctor_time_off
WHERE doctor_id = $1
  AND ends_at >= $2
ORDER BY starts_at ASC;

-- name: ListDoctorTimeOffInRange :many
SELECT id, doctor_id, starts_at, ends_at, reason, created_at
FROM doctor_time_off
WHERE doctor_id = $1
  AND starts_at < $3
  AND ends_at > $2
ORDER BY starts_at ASC;