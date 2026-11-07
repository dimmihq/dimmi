-- migrate: phase=expand
-- What the agent actually runs: an immutable snapshot of an action's
-- definition taken when it was published. Every call names the version
-- it ran, so a later edit never changes what a past call meant. A row
-- is never updated once written.

-- +goose Up
CREATE TABLE ai_action_versions (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    action_id uuid NOT NULL REFERENCES ai_actions ON DELETE CASCADE,
    number integer NOT NULL CHECK (number > 0),
    spec jsonb NOT NULL,
    spec_hash text NOT NULL,
    note text,
    published_by_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX ai_action_versions_workspace_id_action_id_number_key
    ON ai_action_versions (workspace_id, action_id, number);

ALTER TABLE ai_actions ADD CONSTRAINT ai_actions_published_version_id_fkey
    FOREIGN KEY (published_version_id) REFERENCES ai_action_versions ON DELETE SET NULL;

-- +goose StatementBegin
CREATE FUNCTION ai_action_versions_immutable() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'ai_action_versions rows are immutable';
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER ai_action_versions_immutable BEFORE UPDATE ON ai_action_versions
    FOR EACH ROW EXECUTE FUNCTION ai_action_versions_immutable();

ALTER TABLE ai_action_versions ENABLE ROW LEVEL SECURITY;
CREATE POLICY ai_action_versions_tenant ON ai_action_versions
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
ALTER TABLE ai_actions DROP CONSTRAINT ai_actions_published_version_id_fkey;
DROP TABLE ai_action_versions;
DROP FUNCTION ai_action_versions_immutable();
