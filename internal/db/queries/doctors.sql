-- name: CreateDoctor :one 
insert into doctors (account_id, department_id, first_name, last_name)
values($1, $2, $3, $4)
returning account_id, department_id, first_name, last_name;

-- name: ListActiveDoctors :many
SELECT 
    d.first_name, 
    d.last_name,
    d.account_id,
    a.email,
    d.department_id,
    dept.name AS department_name,
    a.created_at
FROM doctors d
JOIN accounts a ON a.id = d.account_id
LEFT JOIN departments dept ON dept.id = d.department_id
WHERE a.status = 'active'
  AND (sqlc.narg('department_id')::int IS NULL OR d.department_id = sqlc.narg('department_id'))
ORDER BY a.email ASC;

-- name: GetDoctorProfileByID :one
SELECT 
    d.first_name, 
    d.last_name,
    d.account_id,
    a.email,
    a.status,
    d.department_id,
    dept.name AS department_name,
    a.created_at
FROM doctors d
JOIN accounts a ON a.id = d.account_id
LEFT JOIN departments dept ON dept.id = d.department_id
WHERE d.account_id = $1
LIMIT 1;

-- name: GetDoctorByAccountID :one
select account_id, department_id
from doctors 
where account_id = $1 limit 1;

