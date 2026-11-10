-- migrate: phase=expand
-- A broadcast is one message sent once to many people on one channel.
-- Topics are the lists people can leave without leaving everything; an
-- absent subscription row means the topic's default. Each recipient is
-- one row, frozen when sending starts, so a retry never sends twice and
-- the report can say what happened to every person or Slack channel.

-- +goose Up
CREATE TABLE broadcast_topics (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
    description text NOT NULL DEFAULT '' CHECK (char_length(description) <= 300),
    consent text NOT NULL CHECK (consent IN ('opt_in', 'opt_out')),
    archived_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX broadcast_topics_workspace_id_name_key ON broadcast_topics (workspace_id, lower(name))
    WHERE archived_at IS NULL;

CREATE TABLE contact_topic_subscriptions (
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    contact_id uuid NOT NULL REFERENCES contacts ON DELETE CASCADE,
    topic_id uuid NOT NULL REFERENCES broadcast_topics ON DELETE CASCADE,
    subscribed boolean NOT NULL,
    source text NOT NULL CHECK (source IN ('preference_page', 'one_click', 'teammate', 'api', 'import')),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, contact_id, topic_id)
);

CREATE TABLE broadcast_opt_outs (
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    contact_id uuid NOT NULL REFERENCES contacts ON DELETE CASCADE,
    source text NOT NULL CHECK (source IN ('preference_page', 'one_click', 'teammate', 'api', 'import')),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, contact_id)
);

CREATE TABLE broadcasts (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 120),
    channel text NOT NULL CHECK (channel IN ('email', 'chat', 'post', 'slack')),
    status text NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'scheduled', 'sending', 'paused', 'sent', 'stopped')),
    pause_reason text,
    audience_id uuid REFERENCES audiences ON DELETE SET NULL,
    rules jsonb CHECK (rules IS NULL OR octet_length(rules::text) <= 8192),
    room_ids uuid[] NOT NULL DEFAULT '{}',
    topic_id uuid REFERENCES broadcast_topics ON DELETE SET NULL,
    service boolean NOT NULL DEFAULT false,
    subject text NOT NULL DEFAULT '' CHECK (char_length(subject) <= 200),
    preview text NOT NULL DEFAULT '' CHECK (char_length(preview) <= 200),
    body text NOT NULL DEFAULT '' CHECK (char_length(body) <= 50000),
    sender_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    email_address_id uuid REFERENCES email_addresses ON DELETE SET NULL,
    slack_as text NOT NULL DEFAULT 'bot' CHECK (slack_as IN ('bot', 'member')),
    reply_team_id uuid REFERENCES teams ON DELETE SET NULL,
    reply_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    ai_replies boolean NOT NULL DEFAULT false,
    track_clicks boolean NOT NULL DEFAULT true,
    skip_frequency_cap boolean NOT NULL DEFAULT false,
    schedule_kind text NOT NULL DEFAULT 'now' CHECK (schedule_kind IN ('now', 'at', 'local')),
    scheduled_at timestamptz,
    goal jsonb,
    counts jsonb NOT NULL DEFAULT '{}',
    created_by_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    sent_by_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    started_at timestamptz,
    finished_at timestamptz,
    deleted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX broadcasts_workspace_id_created_at_index ON broadcasts (workspace_id, created_at DESC)
    WHERE deleted_at IS NULL;
CREATE INDEX broadcasts_due_index ON broadcasts (scheduled_at) WHERE status = 'scheduled';
CREATE INDEX broadcasts_sending_index ON broadcasts (status) WHERE status = 'sending';

CREATE TABLE broadcast_recipients (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    broadcast_id uuid NOT NULL REFERENCES broadcasts ON DELETE CASCADE,
    contact_id uuid REFERENCES contacts ON DELETE SET NULL,
    company_id uuid REFERENCES companies ON DELETE SET NULL,
    room_id uuid REFERENCES channel_rooms ON DELETE SET NULL,
    address text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'queued' CHECK (status IN (
        'queued', 'sent', 'delivered', 'failed', 'bounced', 'complained', 'suppressed', 'skipped', 'recalled'
    )),
    reason text,
    reply_token text NOT NULL,
    message_id_header text,
    outbound_email_id uuid REFERENCES outbound_emails ON DELETE SET NULL,
    external_ref text,
    conversation_id uuid REFERENCES conversations ON DELETE SET NULL,
    attempts integer NOT NULL DEFAULT 0,
    due_at timestamptz NOT NULL DEFAULT now(),
    sent_at timestamptz,
    delivered_at timestamptz,
    opened_at timestamptz,
    clicked_at timestamptz,
    replied_at timestamptz,
    reacted_at timestamptz,
    reactions integer NOT NULL DEFAULT 0,
    unsubscribed_at timestamptz,
    converted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX broadcast_recipients_broadcast_id_contact_id_key ON broadcast_recipients (broadcast_id, contact_id)
    WHERE contact_id IS NOT NULL AND room_id IS NULL;
CREATE UNIQUE INDEX broadcast_recipients_broadcast_id_room_id_key ON broadcast_recipients (broadcast_id, room_id)
    WHERE room_id IS NOT NULL;
CREATE UNIQUE INDEX broadcast_recipients_broadcast_id_address_key ON broadcast_recipients (broadcast_id, lower(address))
    WHERE address <> '' AND status IN ('queued', 'sent', 'delivered');
CREATE UNIQUE INDEX broadcast_recipients_reply_token_key ON broadcast_recipients (reply_token);
CREATE INDEX broadcast_recipients_queue_index ON broadcast_recipients (broadcast_id, due_at)
    WHERE status = 'queued';
CREATE INDEX broadcast_recipients_broadcast_id_status_index ON broadcast_recipients (broadcast_id, status);
CREATE INDEX broadcast_recipients_contact_id_index ON broadcast_recipients (workspace_id, contact_id, sent_at DESC)
    WHERE contact_id IS NOT NULL;
CREATE INDEX broadcast_recipients_message_id_header_index ON broadcast_recipients (workspace_id, message_id_header)
    WHERE message_id_header IS NOT NULL;
CREATE INDEX broadcast_recipients_conversation_id_index ON broadcast_recipients (workspace_id, conversation_id)
    WHERE conversation_id IS NOT NULL;
CREATE INDEX broadcast_recipients_outbound_email_id_index ON broadcast_recipients (workspace_id, outbound_email_id)
    WHERE outbound_email_id IS NOT NULL;
CREATE INDEX broadcast_recipients_external_ref_index ON broadcast_recipients (workspace_id, room_id, external_ref)
    WHERE external_ref IS NOT NULL;

CREATE TABLE broadcast_links (
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    broadcast_id uuid NOT NULL REFERENCES broadcasts ON DELETE CASCADE,
    n integer NOT NULL,
    url text NOT NULL CHECK (char_length(url) <= 2048),
    clicks integer NOT NULL DEFAULT 0,
    PRIMARY KEY (broadcast_id, n)
);

ALTER TABLE broadcast_topics ENABLE ROW LEVEL SECURITY;
CREATE POLICY broadcast_topics_tenant ON broadcast_topics USING (workspace_id = ANY (dimmi_visible_workspaces()));
ALTER TABLE contact_topic_subscriptions ENABLE ROW LEVEL SECURITY;
CREATE POLICY contact_topic_subscriptions_tenant ON contact_topic_subscriptions
    USING (workspace_id = ANY (dimmi_visible_workspaces()));
ALTER TABLE broadcast_opt_outs ENABLE ROW LEVEL SECURITY;
CREATE POLICY broadcast_opt_outs_tenant ON broadcast_opt_outs USING (workspace_id = ANY (dimmi_visible_workspaces()));
ALTER TABLE broadcasts ENABLE ROW LEVEL SECURITY;
CREATE POLICY broadcasts_tenant ON broadcasts USING (workspace_id = ANY (dimmi_visible_workspaces()));
ALTER TABLE broadcast_recipients ENABLE ROW LEVEL SECURITY;
CREATE POLICY broadcast_recipients_tenant ON broadcast_recipients
    USING (workspace_id = ANY (dimmi_visible_workspaces()));
ALTER TABLE broadcast_links ENABLE ROW LEVEL SECURITY;
CREATE POLICY broadcast_links_tenant ON broadcast_links USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE broadcast_links;
DROP TABLE broadcast_recipients;
DROP TABLE broadcasts;
DROP TABLE broadcast_opt_outs;
DROP TABLE contact_topic_subscriptions;
DROP TABLE broadcast_topics;
