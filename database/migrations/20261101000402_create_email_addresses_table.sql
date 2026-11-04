-- migrate: phase=expand
-- An email address is the detail of one email channel connection: the
-- public address customers write to, and the forwarding address on the
-- install's inbound domain that mail really arrives at. The forwarding
-- address is the routing key, so it is unique on the whole install and
-- equals the connection's external id; the public address is only a
-- label and may repeat across workspaces. A workspace has at most one
-- default address, which sends when nothing else is chosen.

-- +goose Up
CREATE TABLE email_addresses (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    channel_connection_id uuid NOT NULL UNIQUE REFERENCES channel_connections ON DELETE CASCADE,
    address citext NOT NULL,
    display_name text NOT NULL CHECK (char_length(display_name) BETWEEN 1 AND 80),
    email_domain_id uuid REFERENCES email_domains ON DELETE SET NULL,
    forwarding_address citext NOT NULL,
    reply_to_mode text NOT NULL DEFAULT 'token' CHECK (reply_to_mode IN ('token', 'none')),
    from_name_style text NOT NULL DEFAULT 'member_and_name'
        CHECK (from_name_style IN ('member_and_name', 'name_only')),
    is_default boolean NOT NULL DEFAULT false,
    settings jsonb NOT NULL DEFAULT '{}',
    forwarding_confirmation jsonb,
    inbound_verified_at timestamptz,
    last_inbound_at timestamptz,
    last_outbound_at timestamptz,
    deleted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX email_addresses_forwarding_address_key ON email_addresses (forwarding_address);
CREATE UNIQUE INDEX email_addresses_workspace_id_address_key ON email_addresses (workspace_id, address)
    WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX email_addresses_workspace_id_default_key ON email_addresses (workspace_id)
    WHERE is_default AND deleted_at IS NULL;

-- +goose Down
DROP TABLE email_addresses;
