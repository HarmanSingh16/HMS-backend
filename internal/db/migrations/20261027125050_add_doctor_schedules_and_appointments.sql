-- +goose Up

-- 1. Enable btree_gist extension (required for the GiST exclusion constraint with doctor_id '=')
CREATE EXTENSION IF NOT EXISTS btree_gist;

-- 2. Doctor Office Hours (weekly recurring schedule)
CREATE TABLE doctor_office_hours (
    id INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    doctor_id INT NOT NULL REFERENCES doctors(account_id) ON DELETE CASCADE,
    weekday SMALLINT NOT NULL CHECK (weekday >= 0 AND weekday <= 6), -- 0 = Sunday, 1 = Monday, ..., 6 = Saturday (matches Go's time.Weekday)
    start_time TIME NOT NULL,
    end_time TIME NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT check_office_hours_duration CHECK (start_time < end_time)
);

CREATE INDEX idx_doctor_office_hours_doctor_weekday ON doctor_office_hours(doctor_id, weekday);

-- 3. Doctor Time Off (scheduled leaves and blackout dates)
CREATE TABLE doctor_time_off (
    id INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    doctor_id INT NOT NULL REFERENCES doctors(account_id) ON DELETE CASCADE,
    starts_at TIMESTAMPTZ NOT NULL,
    ends_at TIMESTAMPTZ NOT NULL,
    reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT check_time_off_duration CHECK (starts_at < ends_at)
);

CREATE INDEX idx_doctor_time_off_doctor_dates ON doctor_time_off(doctor_id, starts_at, ends_at);

-- 4. Appointment Status Enum & Appointments Table
CREATE TYPE appointment_status AS ENUM ('booked', 'checked_in', 'completed', 'cancelled', 'no_show');

CREATE TABLE appointments (
    id INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    doctor_id INT NOT NULL REFERENCES doctors(account_id) ON DELETE CASCADE,
    patient_id INT NOT NULL REFERENCES patients(account_id) ON DELETE CASCADE,
    starts_at TIMESTAMPTZ NOT NULL,
    ends_at TIMESTAMPTZ NOT NULL,
    status appointment_status NOT NULL DEFAULT 'booked',
    cancellation_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT check_appointment_duration CHECK (starts_at < ends_at),
    -- ZERO DOUBLE-BOOKING EXCLUSION CONSTRAINT:
    -- Guarantees no two non-cancelled appointments for the same doctor can ever overlap in time.
    CONSTRAINT no_overlapping_doctor_appointments 
    EXCLUDE USING gist (
        doctor_id WITH =,
        tstzrange(starts_at, ends_at) WITH &&
    ) WHERE (status NOT IN ('cancelled'))
);

CREATE INDEX idx_appointments_doctor_starts_at ON appointments(doctor_id, starts_at);
CREATE INDEX idx_appointments_patient_starts_at ON appointments(patient_id, starts_at);
CREATE INDEX idx_appointments_status ON appointments(status);

-- 5. Appointment Status Transition History
CREATE TABLE appointment_status_events (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    appointment_id INT NOT NULL REFERENCES appointments(id) ON DELETE CASCADE,
    from_status appointment_status,
    to_status appointment_status NOT NULL,
    changed_by INT REFERENCES accounts(id) ON DELETE SET NULL,
    reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_appointment_status_events_appt ON appointment_status_events(appointment_id, created_at ASC);

-- 6. Link notifications outbox to appointments
ALTER TABLE notifications
    ADD CONSTRAINT fk_notifications_appointment 
    FOREIGN KEY (appointment_id) REFERENCES appointments(id) ON DELETE CASCADE;


-- +goose Down
ALTER TABLE notifications 
    DROP CONSTRAINT IF EXISTS fk_notifications_appointment;

DROP TABLE IF EXISTS appointment_status_events;
DROP TABLE IF EXISTS appointments;
DROP TYPE IF EXISTS appointment_status;
DROP TABLE IF EXISTS doctor_time_off;
DROP TABLE IF EXISTS doctor_office_hours;