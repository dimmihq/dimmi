-- migrate: phase=expand
-- A spreadsheet of customers or companies a teammate brings in. The file
-- is kept with the import so a preview, a dry run and the run itself all
-- read the same rows. Mapping says which column fills which field, mode
-- whether rows that match someone already there change them or are left
-- alone. The run happens in the background and counts as it goes; rows
-- that fail are kept with why, for a report the teammate downloads.

-- +goose Up
CREATE TABLE imports (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    kind text NOT NULL CHECK (kind IN ('contacts', 'companies')),
    status text NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'queued', 'running', 'completed', 'failed')),
    file_name text NOT NULL,
    file bytea NOT NULL,
    columns jsonb NOT NULL DEFAULT '[]',
    row_count integer NOT NULL DEFAULT 0,
    mapping jsonb NOT NULL DEFAULT '[]',
    mode text NOT NULL DEFAULT 'update' CHECK (mode IN ('update', 'skip')),
    processed_rows integer NOT NULL DEFAULT 0,
    created_count integer NOT NULL DEFAULT 0,
    updated_count integer NOT NULL DEFAULT 0,
    skipped_count integer NOT NULL DEFAULT 0,
    failed_count integer NOT NULL DEFAULT 0,
    row_errors jsonb NOT NULL DEFAULT '[]',
    error text,
    created_by_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    started_at timestamptz,
    finished_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX imports_workspace_id_kind_created_at_index ON imports (workspace_id, kind, created_at DESC);

ALTER TABLE imports ENABLE ROW LEVEL SECURITY;
CREATE POLICY imports_tenant ON imports USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE imports;
