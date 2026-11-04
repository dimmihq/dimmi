-- migrate: phase=expand
-- A personal access token acts as one member of one workspace. The
-- framework stores the workspace as text in tenant_id; workspace_id is
-- derived from it so the table carries a real tenant column, with its
-- foreign key, index and row level security, without the token store
-- knowing about workspaces. The owner is the member the token acts as.

-- +goose NO TRANSACTION
-- +goose Up
ALTER TABLE personal_access_tokens
    ADD COLUMN workspace_id uuid GENERATED ALWAYS AS (tenant_id::uuid) STORED,
    ADD COLUMN expiry_notified_at timestamptz,
    ADD COLUMN revoked_by_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    ADD COLUMN minted_as_role text,
    ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now(),
    ADD CONSTRAINT personal_access_tokens_workspace_id_fkey
        FOREIGN KEY (workspace_id) REFERENCES workspaces ON DELETE CASCADE,
    ADD CONSTRAINT personal_access_tokens_owner_id_fkey
        FOREIGN KEY (owner_id) REFERENCES workspace_members ON DELETE CASCADE,
    ADD CONSTRAINT personal_access_tokens_workspace_id_not_null CHECK (workspace_id IS NOT NULL) NOT VALID;

ALTER TABLE personal_access_tokens VALIDATE CONSTRAINT personal_access_tokens_workspace_id_not_null;

CREATE INDEX CONCURRENTLY IF NOT EXISTS personal_access_tokens_workspace_id_owner_id_created_at_index
    ON personal_access_tokens (workspace_id, owner_id, created_at DESC);
CREATE INDEX CONCURRENTLY IF NOT EXISTS personal_access_tokens_workspace_id_live_index
    ON personal_access_tokens (workspace_id, created_at DESC) WHERE revoked_at IS NULL;
CREATE INDEX CONCURRENTLY IF NOT EXISTS personal_access_tokens_expires_at_index
    ON personal_access_tokens (expires_at) WHERE revoked_at IS NULL AND expires_at IS NOT NULL;

ALTER TABLE personal_access_tokens ENABLE ROW LEVEL SECURITY;
CREATE POLICY personal_access_tokens_tenant ON personal_access_tokens
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP POLICY personal_access_tokens_tenant ON personal_access_tokens;
ALTER TABLE personal_access_tokens DISABLE ROW LEVEL SECURITY;
DROP INDEX IF EXISTS personal_access_tokens_expires_at_index;
DROP INDEX IF EXISTS personal_access_tokens_workspace_id_live_index;
DROP INDEX IF EXISTS personal_access_tokens_workspace_id_owner_id_created_at_index;
ALTER TABLE personal_access_tokens
    DROP CONSTRAINT personal_access_tokens_workspace_id_not_null,
    DROP CONSTRAINT personal_access_tokens_owner_id_fkey,
    DROP CONSTRAINT personal_access_tokens_workspace_id_fkey,
    DROP COLUMN updated_at,
    DROP COLUMN minted_as_role,
    DROP COLUMN revoked_by_member_id,
    DROP COLUMN expiry_notified_at,
    DROP COLUMN workspace_id;
