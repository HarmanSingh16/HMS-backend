-- name: CreatePatient :one
INSERT INTO patients (
    account_id,
    first_name,
    last_name,
    date_of_birth,
    phone_number,
    gender,
    address
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING account_id, first_name, last_name, date_of_birth, phone_number, gender, address, created_at, updated_at;

-- name: GetPatientByAccountID :one
-- Retrieves full patient demographics joined with account credentials and status
SELECT 
    p.account_id,
    a.email,
    a.role,
    a.status AS account_status,
    p.first_name,
    p.last_name,
    p.date_of_birth,
    p.phone_number,
    p.gender,
    p.address,
    p.created_at,
    p.updated_at
FROM patients p
JOIN accounts a ON a.id = p.account_id
WHERE p.account_id = $1
LIMIT 1;

-- name: UpdatePatientDemographics :one
-- Allows updating demographic contact information
UPDATE patients
SET 
    first_name = $2,
    last_name = $3,
    date_of_birth = $4,
    phone_number = $5,
    gender = $6,
    address = $7,
    updated_at = NOW()
WHERE account_id = $1
RETURNING account_id, first_name, last_name, date_of_birth, phone_number, gender, address, created_at, updated_at;

-- name: SearchPatients :many
-- Fast multi-field search across name, email, and phone with pagination
SELECT 
    p.account_id,
    a.email,
    a.role,
    a.status AS account_status,
    p.first_name,
    p.last_name,
    p.date_of_birth,
    p.phone_number,
    p.gender,
    p.address,
    p.created_at,
    p.updated_at
FROM patients p
JOIN accounts a ON a.id = p.account_id
WHERE 
    ($1::text = '' OR 
     p.first_name ILIKE '%' || $1 || '%' OR 
     p.last_name ILIKE '%' || $1 || '%' OR 
     a.email ILIKE '%' || $1 || '%' OR 
     p.phone_number ILIKE '%' || $1 || '%')
ORDER BY p.created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountPatients :one
-- Used for pagination metadata when searching
SELECT COUNT(*)
FROM patients p
JOIN accounts a ON a.id = p.account_id
WHERE 
    ($1::text = '' OR 
     p.first_name ILIKE '%' || $1 || '%' OR 
     p.last_name ILIKE '%' || $1 || '%' OR 
     a.email ILIKE '%' || $1 || '%' OR 
     p.phone_number ILIKE '%' || $1 || '%');