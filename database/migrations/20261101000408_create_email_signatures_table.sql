-- migrate: phase=expand
-- A signature closes a member's email replies: one for all addresses,
-- and optionally one per address that wins over it. It is short
-- Markdown with links and emphasis only.

-- +goose Up
CREATE TABLE email_signatures (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    workspace_member_id uuid NOT NULL REFERENCES workspace_members ON DELETE CASCADE,
    email_address_id uuid REFERENCES email_addresses ON DELETE CASCADE,
    body text NOT NULL CHECK (char_length(body) <= 2000),
    enabled boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT email_signatures_workspace_member_id_email_address_id_key
        UNIQUE NULLS NOT DISTINCT (workspace_member_id, email_address_id)
);

-- +goose Down
DROP TABLE email_signatures;
