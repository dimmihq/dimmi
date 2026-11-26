-- migrate: phase=expand
-- A migration brings a workspace's history in from another tool through
-- dimmi's own import format, whichever tool it was. It belongs to one
-- account of that tool, so starting again for the same account resumes
-- it. Every record it receives is kept under the id it had there: what
-- it became here, a hash of what was sent so an unchanged resend is
-- skipped, and, for a record whose references have not arrived yet, the
-- record itself until they do. A conversation also keeps what the
-- migration last wrote to it, so a later sync appends new entries and
-- leaves alone what a teammate changed here since. An undo removes
-- everything the migration created.

-- +goose Up
CREATE TABLE migrations (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    source text NOT NULL CHECK (source ~ '^[a-z0-9][a-z0-9_-]{0,39}$'),
    source_account text NOT NULL CHECK (char_length(source_account) BETWEEN 1 AND 255),
    name text CHECK (char_length(name) <= 120),
    format_version integer NOT NULL DEFAULT 1,
    status text NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'rolling_back', 'rolled_back', 'rollback_failed')),
    created_by_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    last_record_at timestamptz,
    rollback_by_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    rollback_requested_at timestamptz,
    rollback_finished_at timestamptz,
    rollback_removed integer NOT NULL DEFAULT 0,
    rollback_error text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX migrations_workspace_id_source_account_key ON migrations (workspace_id, source, source_account)
    WHERE status <> 'rolled_back';
CREATE INDEX migrations_workspace_id_created_at_index ON migrations (workspace_id, created_at DESC);
CREATE INDEX migrations_created_by_member_id_index ON migrations (created_by_member_id);
CREATE INDEX migrations_rollback_by_member_id_index ON migrations (rollback_by_member_id);

ALTER TABLE migrations ENABLE ROW LEVEL SECURITY;
CREATE POLICY migrations_tenant ON migrations USING (workspace_id = ANY (dimmi_visible_workspaces()));

CREATE TABLE migration_records (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    migration_id uuid NOT NULL REFERENCES migrations ON DELETE CASCADE,
    kind text NOT NULL CHECK (char_length(kind) BETWEEN 1 AND 40),
    source_id text NOT NULL CHECK (char_length(source_id) BETWEEN 1 AND 255),
    status text NOT NULL CHECK (status IN ('applied', 'deferred', 'failed', 'unsupported')),
    record_id uuid,
    created boolean NOT NULL DEFAULT false,
    label text CHECK (char_length(label) <= 200),
    content_hash text NOT NULL DEFAULT '',
    source_updated_at timestamptz,
    payload jsonb,
    waiting_for jsonb NOT NULL DEFAULT '[]' CHECK (jsonb_typeof(waiting_for) = 'array'),
    state jsonb NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(state) = 'object'),
    error jsonb,
    warnings jsonb NOT NULL DEFAULT '[]' CHECK (jsonb_typeof(warnings) = 'array'),
    conflicts jsonb NOT NULL DEFAULT '[]' CHECK (jsonb_typeof(conflicts) = 'array'),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT migration_records_migration_id_kind_source_id_key UNIQUE (migration_id, kind, source_id)
);

CREATE INDEX migration_records_migration_id_status_index ON migration_records (migration_id, status, updated_at DESC);
CREATE INDEX migration_records_workspace_id_record_id_index ON migration_records (workspace_id, record_id)
    WHERE record_id IS NOT NULL;

ALTER TABLE migration_records ENABLE ROW LEVEL SECURITY;
CREATE POLICY migration_records_tenant ON migration_records
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE migration_records;
DROP TABLE migrations;
