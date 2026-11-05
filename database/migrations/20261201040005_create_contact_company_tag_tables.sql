-- migrate: phase=expand
-- People and companies carry the same tags as conversations, so a label
-- means one thing wherever it shows. The tag follows the person across
-- their conversations. Each link records who added it and carries its
-- workspace so row level security covers it alone.

-- +goose Up
CREATE TABLE contact_tag (
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    contact_id uuid NOT NULL REFERENCES contacts ON DELETE CASCADE,
    tag_id uuid NOT NULL REFERENCES tags ON DELETE CASCADE,
    created_by_type text NOT NULL CHECK (created_by_type ~ '^[a-z][a-z0-9_]{1,39}$'),
    created_by_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (contact_id, tag_id)
);

CREATE INDEX contact_tag_workspace_id_tag_id_index ON contact_tag (workspace_id, tag_id);

CREATE TABLE company_tag (
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    company_id uuid NOT NULL REFERENCES companies ON DELETE CASCADE,
    tag_id uuid NOT NULL REFERENCES tags ON DELETE CASCADE,
    created_by_type text NOT NULL CHECK (created_by_type ~ '^[a-z][a-z0-9_]{1,39}$'),
    created_by_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (company_id, tag_id)
);

CREATE INDEX company_tag_workspace_id_tag_id_index ON company_tag (workspace_id, tag_id);

ALTER TABLE contact_tag ENABLE ROW LEVEL SECURITY;
CREATE POLICY contact_tag_tenant ON contact_tag USING (workspace_id = ANY (dimmi_visible_workspaces()));

ALTER TABLE company_tag ENABLE ROW LEVEL SECURITY;
CREATE POLICY company_tag_tenant ON company_tag USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE company_tag;
DROP TABLE contact_tag;
