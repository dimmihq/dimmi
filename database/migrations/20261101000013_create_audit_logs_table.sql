-- migrate: phase=expand
-- The audit log records who changed what in a workspace. It is written
-- once and never edited: the application's role loses UPDATE on it, and
-- only the retention job deletes. Actor and subject labels are copied at
-- write time so an entry still reads right after a member or token is
-- gone. Actor type and channel are registry values shared with every
-- other spec's actors.

-- +goose Up
CREATE TABLE audit_logs (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    event text NOT NULL,
    actor_type text NOT NULL CHECK (actor_type ~ '^[a-z][a-z0-9_]{1,39}$'),
    actor_id uuid,
    via_kind text CHECK (via_kind ~ '^[a-z][a-z0-9_]{1,39}$'),
    via_id uuid,
    actor_label text NOT NULL,
    subject_type text,
    subject_id uuid,
    subject_label text,
    changes jsonb NOT NULL DEFAULT '{}',
    ip inet,
    user_agent text,
    request_id text,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX audit_logs_workspace_id_created_at_id_index ON audit_logs (workspace_id, created_at DESC, id DESC);
CREATE INDEX audit_logs_workspace_id_event_created_at_index ON audit_logs (workspace_id, event, created_at DESC);
CREATE INDEX audit_logs_workspace_id_actor_id_created_at_index ON audit_logs (workspace_id, actor_id, created_at DESC);
CREATE INDEX audit_logs_workspace_id_via_created_at_index
    ON audit_logs (workspace_id, via_kind, via_id, created_at DESC);
CREATE INDEX audit_logs_workspace_id_subject_created_at_index
    ON audit_logs (workspace_id, subject_type, subject_id, created_at DESC);

ALTER TABLE audit_logs ENABLE ROW LEVEL SECURITY;
CREATE POLICY audit_logs_tenant ON audit_logs USING (workspace_id = ANY (dimmi_visible_workspaces()));

REVOKE UPDATE ON audit_logs FROM CURRENT_USER;

-- +goose Down
DROP TABLE audit_logs;
