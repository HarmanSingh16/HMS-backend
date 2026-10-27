-- +goose Up
CREATE TYPE user_role as ENUM ('admin', 'doctor', 'patient', 'receptionist');
CREATE TYPE account_status AS ENUM ('invited', 'active', 'deactivated');

CREATE TABLE accounts (
  id INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  email varchar(150) NOT NULL UNIQUE,
  password_hash text,
  role user_role NOT NULL, 
  status account_status NOT NULL DEFAULT 'invited', 
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE invites (
  id INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  account_id INT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  token_hash VARCHAR(255) NOT NULL UNIQUE,
  expires_at TIMESTAMPTZ NOT NULL,
  claimed_at TIMESTAMPTZ, -- can be null until claimed
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE departments (
  id INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  name varchar(100)
);

CREATE TABLE doctors (
  account_id INT PRIMARY KEY REFERENCES accounts(id),
  department_id INT REFERENCES departments(id)
);

CREATE TABLE patients (
  account_id INT PRIMARY KEY REFERENCES accounts(id),
  date_of_birth DATE
);


-- +goose Down
DROP TABLE IF EXISTS patients;
DROP TABLE IF EXISTS doctors;
DROP TABLE IF EXISTS departments;
DROP TABLE IF EXISTS invites;
DROP TABLE IF EXISTS accounts;
DROP TYPE IF EXISTS account_status;
DROP TYPE IF EXISTS user_role;
