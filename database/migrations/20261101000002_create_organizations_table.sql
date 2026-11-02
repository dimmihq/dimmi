-- migrate: phase=expand
-- lint:global
-- An organization is the anchor for everything that will span
-- workspaces later: single sign-on, billing ownership, org-wide policy.
-- Today every workspace gets exactly one, created in the same
-- transaction, and no API shows it. It is no tenant's row, so it has no
-- row level security; code reaches it only through its workspace.

-- +goose Up
CREATE TABLE organizations (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    name text NOT NULL,
    created_by_user_id uuid REFERENCES users ON DELETE SET NULL,
    settings jsonb NOT NULL DEFAULT '{}',
    deleted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE organizations;
