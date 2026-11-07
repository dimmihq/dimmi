-- migrate: phase=expand
-- A version is a published workflow, frozen: runs point at the version
-- they started on for as long as they last, so a version is never
-- changed. compiled holds the engine's index of the steps and every id
-- the definition references.

-- +goose Up
CREATE TABLE workflow_versions (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    workflow_id uuid NOT NULL REFERENCES workflows ON DELETE CASCADE,
    number integer NOT NULL CHECK (number > 0),
    spec jsonb NOT NULL,
    spec_hash text NOT NULL,
    kind text NOT NULL CHECK (kind IN ('customer_facing', 'background')),
    compiled jsonb NOT NULL,
    note text CHECK (char_length(note) <= 280),
    published_by_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    published_via text,
    restored_from_version_id uuid REFERENCES workflow_versions ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX workflow_versions_workflow_id_number_key ON workflow_versions (workflow_id, number);
CREATE INDEX workflow_versions_workspace_id_workflow_id_number_index
    ON workflow_versions (workspace_id, workflow_id, number DESC);

ALTER TABLE workflows ADD CONSTRAINT workflows_live_version_id_fkey
    FOREIGN KEY (live_version_id) REFERENCES workflow_versions ON DELETE SET NULL;

-- +goose StatementBegin
CREATE FUNCTION workflow_versions_refuse_update() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.spec IS DISTINCT FROM OLD.spec OR NEW.compiled IS DISTINCT FROM OLD.compiled
        OR NEW.number IS DISTINCT FROM OLD.number OR NEW.workflow_id IS DISTINCT FROM OLD.workflow_id THEN
        RAISE EXCEPTION 'workflow versions are immutable';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER workflow_versions_immutable BEFORE UPDATE ON workflow_versions
    FOR EACH ROW EXECUTE FUNCTION workflow_versions_refuse_update();

ALTER TABLE workflow_versions ENABLE ROW LEVEL SECURITY;
CREATE POLICY workflow_versions_tenant ON workflow_versions
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
ALTER TABLE workflows DROP CONSTRAINT workflows_live_version_id_fkey;
DROP TABLE workflow_versions;
DROP FUNCTION workflow_versions_refuse_update();
