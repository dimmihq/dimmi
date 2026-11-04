-- migrate: phase=expand
-- A workspace's standing as a sender: whether it may send, and the caps
-- that apply to it. A paused workspace sends nothing until someone
-- resumes it. The row is created the first time it is needed.

-- +goose Up
CREATE TABLE email_sending_profiles (
    workspace_id uuid PRIMARY KEY REFERENCES workspaces ON DELETE CASCADE,
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'paused', 'review')),
    paused_at timestamptz,
    paused_reason text,
    daily_limit integer,
    hourly_limit integer,
    ses_tenant_name text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE email_sending_profiles;
