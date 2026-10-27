-- name: ListDepartments :many
SELECT id, name
from departments
order by name asc;

-- name: GetDepartmentById :one
select id, name 
from departments
where id = $1 LIMIT 1;

-- name: GetDepartmentByName :one
select id, name 
from departments
where lower(name) = Lower($1) Limit 1;

-- name: CreateDepartment :one
INSERT INTO DEPARTMENTS(name)
values($1)
returning id, name;

-- name: UpdateDepartment :one
update departments 
set name = $2 
where id = $1
returning id, name;