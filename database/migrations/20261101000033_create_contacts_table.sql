-- migrate: phase=expand
-- A contact is one customer. A user is someone the host app identified;
-- a lead is anyone else, and an email a lead typed proves nothing, so
-- addresses are not unique and email_verified only turns true from a
-- source that proved it. trusted_attribute_keys names the custom
-- attributes whose current value came from a trusted source, the only
-- ones automated actions may rely on. A merged contact stays as a
-- tombstone pointing at the survivor, so old links keep resolving.

-- +goose Up
CREATE TABLE contacts (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    kind text NOT NULL CHECK (kind IN ('user', 'lead')),
    external_id text,
    email citext,
    email_verified boolean NOT NULL DEFAULT false,
    name text,
    phone text,
    avatar_url text,
    company_id uuid REFERENCES companies ON DELETE SET NULL,
    custom_attributes jsonb NOT NULL DEFAULT '{}',
    trusted_attribute_keys text[] NOT NULL DEFAULT '{}',
    locale text,
    timezone text,
    location jsonb,
    first_seen_at timestamptz,
    last_seen_at timestamptz,
    blocked_at timestamptz,
    merged_into_id uuid REFERENCES contacts ON DELETE SET NULL,
    search tsvector GENERATED ALWAYS AS (
        to_tsvector('simple', coalesce(name, '') || ' ' || coalesce(email::text, '') || ' '
            || coalesce(phone, '') || ' ' || coalesce(external_id, ''))
    ) STORED,
    deleted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX contacts_workspace_id_external_id_key ON contacts (workspace_id, external_id)
    WHERE external_id IS NOT NULL;
CREATE INDEX contacts_workspace_id_email_index ON contacts (workspace_id, email);

ALTER TABLE contacts ENABLE ROW LEVEL SECURITY;
CREATE POLICY contacts_tenant ON contacts USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE contacts;
