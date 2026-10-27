-- +goose Up
ALTER TABLE departments
    ALTER COLUMN name SET NOT NULL, 
    ADD CONSTRAINT departments_name_unique UNIQUE (name);

CREATE TABLE audit_logs(
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    actor_id INT REFERENCES accounts(id) ON DELETE SET NULL, 
    action VARCHAR(100) NOT NULL, 
    entity_type VARCHAR(50) NOT NULL, 
    entity_id INT, 
    details JSONB, 
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_logs_actor_id ON audit_logs(actor_id);
CREATE INDEX idx_audit_logs_action ON audit_logs(action);
CREATE INDEX idx_audit_logs_created_at ON audit_logs(created_at DESC);

CREATE TABLE notifications (
    id INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    recipient_id INT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    invite_id INT REFERENCES invites(id) ON DELETE CASCADE,
    appointment_id INT, -- nullable, populated in appointment scheduling
    type VARCHAR(50) NOT NULL, -- 'account_invite', 'appointment_reminder', 'slot_removed'
    status VARCHAR(20) NOT NULL DEFAULT 'pending', -- 'pending', 'sent', 'failed', 'cancelled'
    payload JSONB,
    scheduled_for TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    sent_at TIMESTAMPTZ,
    retry_count INT NOT NULL DEFAULT 0,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_notifications_outbox ON notifications(status, scheduled_for ASC);


-- +goose Down
DROP TABLE IF EXISTS notifications;
DROP TABLE IF EXISTS audit_logs;
ALTER TABLE departments 
    DROP CONSTRAINT IF EXISTS departments_name_unique,
    ALTER COLUMN name DROP NOT NULL;
