-- migrate: phase=expand
-- People who asked to hear about a help center's changelog by email. An
-- address someone typed stays pending until its owner confirms from the
-- mail, so nobody is subscribed by a stranger; a signed-in reader is
-- active at once. The confirm and unsubscribe links carry a keyed hash
-- of the subscriber and its nonce; a new nonce kills every old link.

-- +goose Up
CREATE TABLE changelog_subscribers (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    help_center_id uuid NOT NULL REFERENCES help_centers ON DELETE CASCADE,
    email citext NOT NULL CHECK (char_length(email) <= 320),
    contact_id uuid REFERENCES contacts ON DELETE SET NULL,
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'active', 'unsubscribed')),
    nonce text NOT NULL,
    confirmation_sent_at timestamptz,
    confirmed_at timestamptz,
    unsubscribed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX changelog_subscribers_help_center_id_email_key ON changelog_subscribers (help_center_id, email);
CREATE INDEX changelog_subscribers_workspace_id_help_center_id_status_index
    ON changelog_subscribers (workspace_id, help_center_id, status, created_at DESC);

ALTER TABLE changelog_subscribers ENABLE ROW LEVEL SECURITY;
CREATE POLICY changelog_subscribers_tenant ON changelog_subscribers
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE changelog_subscribers;
