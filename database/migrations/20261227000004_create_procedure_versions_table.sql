-- migrate: phase=expand
-- Every publish of a procedure is kept as an immutable version: what it
-- said, who published it and why, so the team can compare two versions,
-- see how each did, and put an old one back. The procedure names the
-- version dimmi follows. Its description is also kept as a vector, so a
-- workspace with many procedures offers dimmi the few closest to what the
-- customer asks.

-- +goose Up
CREATE TABLE procedure_versions (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    procedure_id uuid NOT NULL REFERENCES procedures ON DELETE CASCADE,
    version integer NOT NULL CHECK (version > 0),
    name text NOT NULL,
    definition jsonb NOT NULL CHECK (octet_length(definition::text) <= 65536),
    change_note text CHECK (char_length(change_note) <= 500),
    published_by_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    published_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (procedure_id, version)
);

CREATE INDEX procedure_versions_workspace_id_procedure_id_index
    ON procedure_versions (workspace_id, procedure_id, version DESC);

ALTER TABLE procedure_versions ENABLE ROW LEVEL SECURITY;
CREATE POLICY procedure_versions_tenant ON procedure_versions
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

ALTER TABLE procedures
    ADD COLUMN live_version_id uuid REFERENCES procedure_versions ON DELETE SET NULL,
    ADD COLUMN description_embedding halfvec,
    ADD COLUMN embedding_model_key text;

ALTER TABLE conversation_procedure_states
    ADD CONSTRAINT conversation_procedure_states_procedure_version_id_fkey
    FOREIGN KEY (procedure_version_id) REFERENCES procedure_versions ON DELETE SET NULL;

-- +goose Down
ALTER TABLE conversation_procedure_states DROP CONSTRAINT conversation_procedure_states_procedure_version_id_fkey;
ALTER TABLE procedures
    DROP COLUMN live_version_id,
    DROP COLUMN description_embedding,
    DROP COLUMN embedding_model_key;
DROP TABLE procedure_versions;
