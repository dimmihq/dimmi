-- The notifications table as the migration step creates it under its
-- default name, read by sqlc so application queries can use it. The
-- step in migration.go is what runs; a test keeps the two in step.
CREATE TABLE notifications (
    id uuid PRIMARY KEY,
    notifiable_type text NOT NULL,
    notifiable_id text NOT NULL,
    type text NOT NULL,
    data jsonb NOT NULL DEFAULT '{}',
    read_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    tenant_id text,
    group_key text,
    count integer NOT NULL DEFAULT 1,
    last_delivery_id text,
    seen_at timestamptz,
    updated_at timestamptz NOT NULL DEFAULT now()
);
