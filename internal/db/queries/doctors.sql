-- name: CreateDoctor :one 
insert into doctors (account_id, department_id)
values($1, $2)
returning account_id, department_id;

-- name: GetDoctorByAccountID :one
select account_id, department_id 
from doctors 
where account_id = $1 limit 1;