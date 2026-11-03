-- migrate: phase=expand
-- A conversation is one thread with a customer, started on one channel.
-- Its number is the human reference, #1042, handed out per workspace
-- from workspaces.next_conversation_number in the same transaction, so
-- numbers never repeat within a workspace. Spam is a flag beside the
-- status, so taking a conversation out of spam restores where it stood.
-- A snoozed conversation always says when it wakes. While a thread is
-- not closed, the channel's own thread key finds exactly one
-- conversation, so a reply on the channel joins the thread it belongs to.

-- +goose Up
CREATE TABLE conversations (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    number bigint NOT NULL,
    inbox_id uuid NOT NULL REFERENCES inboxes,
    channel_connection_id uuid NOT NULL REFERENCES channel_connections,
    contact_id uuid NOT NULL REFERENCES contacts,
    subject text,
    status text NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'snoozed', 'closed')),
    spam boolean NOT NULL DEFAULT false,
    priority boolean NOT NULL DEFAULT false,
    assignee_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    assignee_team_id uuid REFERENCES teams ON DELETE SET NULL,
    ai_state text NOT NULL DEFAULT 'off' CHECK (ai_state IN ('off', 'active', 'handed_off')),
    snoozed_until timestamptz,
    external_thread_key text,
    source jsonb NOT NULL DEFAULT '{}',
    first_customer_message_at timestamptz,
    last_customer_message_at timestamptz,
    first_reply_at timestamptz,
    last_reply_at timestamptz,
    last_message_at timestamptz,
    closed_at timestamptz,
    message_count integer NOT NULL DEFAULT 0,
    deleted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT conversations_workspace_id_number_key UNIQUE (workspace_id, number),
    CONSTRAINT conversations_snoozed_until_check CHECK (status <> 'snoozed' OR snoozed_until IS NOT NULL)
);

CREATE UNIQUE INDEX conversations_channel_connection_id_external_thread_key_key
    ON conversations (channel_connection_id, external_thread_key)
    WHERE status <> 'closed' AND external_thread_key IS NOT NULL;
CREATE INDEX conversations_workspace_id_status_last_message_at_index
    ON conversations (workspace_id, status, last_message_at DESC);
CREATE INDEX conversations_workspace_id_assignee_member_id_status_index
    ON conversations (workspace_id, assignee_member_id, status);
CREATE INDEX conversations_workspace_id_contact_id_created_at_index
    ON conversations (workspace_id, contact_id, created_at DESC);
CREATE INDEX conversations_workspace_id_inbox_id_status_index ON conversations (workspace_id, inbox_id, status);
CREATE INDEX conversations_workspace_id_created_at_id_index ON conversations (workspace_id, created_at DESC, id DESC);

ALTER TABLE conversations ENABLE ROW LEVEL SECURITY;
CREATE POLICY conversations_tenant ON conversations USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE conversations;
