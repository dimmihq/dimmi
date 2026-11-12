-- migrate: phase=expand
-- A ticket import brings a workspace's tickets in from Intercom or
-- Zendesk: their types with fields, their states with the labels
-- customers saw, the tickets themselves and each one's history. It
-- counts what it made and what it found already there, and keeps the
-- records it touched so a rerun finds them and an undo knows what it
-- made: an undo trashes the tickets the import created and archives
-- the types, fields and states it added that nothing else uses.

-- +goose Up
CREATE TABLE ticket_imports (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    source text NOT NULL CHECK (source IN ('intercom', 'zendesk')),
    status text NOT NULL DEFAULT 'running' CHECK (status IN ('running', 'completed', 'failed')),
    types_created integer NOT NULL DEFAULT 0,
    fields_created integer NOT NULL DEFAULT 0,
    states_created integer NOT NULL DEFAULT 0,
    tickets_created integer NOT NULL DEFAULT 0,
    tickets_found integer NOT NULL DEFAULT 0,
    tickets_failed integer NOT NULL DEFAULT 0,
    links_created integer NOT NULL DEFAULT 0,
    row_errors jsonb NOT NULL DEFAULT '[]' CHECK (jsonb_typeof(row_errors) = 'array'),
    error text,
    created_by_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    started_at timestamptz NOT NULL DEFAULT now(),
    finished_at timestamptz,
    rollback_status text CHECK (rollback_status IN ('queued', 'running', 'completed', 'failed')),
    rollback_by_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    rollback_trashed integer NOT NULL DEFAULT 0,
    rollback_archived integer NOT NULL DEFAULT 0,
    rollback_error text,
    rollback_requested_at timestamptz,
    rollback_finished_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ticket_imports_workspace_id_created_at_index ON ticket_imports (workspace_id, created_at DESC);
CREATE INDEX ticket_imports_created_by_member_id_index ON ticket_imports (created_by_member_id);
CREATE INDEX ticket_imports_rollback_by_member_id_index ON ticket_imports (rollback_by_member_id);

ALTER TABLE ticket_imports ENABLE ROW LEVEL SECURITY;
CREATE POLICY ticket_imports_tenant ON ticket_imports USING (workspace_id = ANY (dimmi_visible_workspaces()));

CREATE TABLE ticket_import_records (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    import_id uuid NOT NULL REFERENCES ticket_imports ON DELETE CASCADE,
    record_kind text NOT NULL CHECK (record_kind IN ('type', 'field', 'state', 'ticket', 'link')),
    external_id text NOT NULL CHECK (char_length(external_id) BETWEEN 1 AND 255),
    record_id uuid NOT NULL,
    created boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ticket_import_records_import_id_record_kind_external_id_key
        UNIQUE (import_id, record_kind, external_id)
);

CREATE INDEX ticket_import_records_workspace_id_index ON ticket_import_records (workspace_id);

ALTER TABLE ticket_import_records ENABLE ROW LEVEL SECURITY;
CREATE POLICY ticket_import_records_tenant ON ticket_import_records
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE ticket_import_records;
DROP TABLE ticket_imports;
