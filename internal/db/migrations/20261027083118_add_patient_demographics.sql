-- +goose Up
-- 1. Alter patients table to add demographic fields
ALTER TABLE patients
    ADD COLUMN first_name VARCHAR(100) NOT NULL,
    ADD COLUMN last_name VARCHAR(100) NOT NULL,
    ADD COLUMN phone_number VARCHAR(30) NOT NULL,
    ADD COLUMN gender VARCHAR(20),
    ADD COLUMN address TEXT,
    ADD COLUMN created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

-- 2. Make date_of_birth NOT NULL (required for hospital records)
ALTER TABLE patients
    ALTER COLUMN date_of_birth SET NOT NULL;

-- 3. Add indexes for high-performance patient search and lookups
-- Fast lookup by phone number (receptionist intake flow)
CREATE INDEX idx_patients_phone_number ON patients(phone_number);

-- Composite index for name searches and sorted directory listings
CREATE INDEX idx_patients_name ON patients(last_name, first_name);


-- +goose Down
-- Revert indexes
DROP INDEX IF EXISTS idx_patients_name;
DROP INDEX IF EXISTS idx_patients_phone_number;

-- Revert columns
ALTER TABLE patients
    ALTER COLUMN date_of_birth DROP NOT NULL,
    DROP COLUMN IF EXISTS updated_at,
    DROP COLUMN IF EXISTS created_at,
    DROP COLUMN IF EXISTS address,
    DROP COLUMN IF EXISTS gender,
    DROP COLUMN IF EXISTS phone_number,
    DROP COLUMN IF EXISTS last_name,
    DROP COLUMN IF EXISTS first_name;