-- migrate: phase=expand
-- The facts reports are read from, each worked out again from scratch
-- for one conversation whenever anything about it changes, so a reopen,
-- a backdated import, a merge or a new schedule never leaves a total to
-- patch by hand. Durations are kept in calendar seconds and in seconds
-- the inbox's schedule was open. Spam, test, deleted and merged-away
-- conversations have no facts.
--
-- conversation_facts is one row per conversation; the assignee and team
-- are the current ones, so who may see a conversation decides who may
-- count it. conversation_reply_facts is one row per customer turn: from
-- the first customer message nobody had answered to the next teammate
-- reply. conversation_member_facts is one row per teammate who replied
-- to or held the conversation.
--
-- backlog_snapshots counts, each day, the conversations still waiting
-- at the end of the workspace's day, by inbox and team: the one number
-- that cannot be worked out later as it stood.

-- +goose Up
CREATE TABLE conversation_facts (
    conversation_id uuid PRIMARY KEY REFERENCES conversations ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    number bigint NOT NULL,
    inbox_id uuid NOT NULL,
    channel text NOT NULL,
    assignee_member_id uuid,
    assignee_team_id uuid,
    priority smallint NOT NULL DEFAULT 0,
    status text NOT NULL,
    imported boolean NOT NULL DEFAULT false,
    started_at timestamptz NOT NULL,
    customer_messages integer NOT NULL DEFAULT 0,
    last_customer_message_at timestamptz,
    first_response_at timestamptz,
    first_response_seconds bigint,
    first_response_business_seconds bigint,
    first_responder_id uuid,
    first_answer_at timestamptz,
    first_answer_seconds bigint,
    first_answer_business_seconds bigint,
    human_replies integer NOT NULL DEFAULT 0,
    first_closed_at timestamptz,
    first_close_seconds bigint,
    first_close_business_seconds bigint,
    closed_at timestamptz,
    resolution_seconds bigint,
    resolution_business_seconds bigint,
    closes integer NOT NULL DEFAULT 0,
    reopens integer NOT NULL DEFAULT 0,
    closed_by_member_id uuid,
    closed_by_team_id uuid,
    first_contact_resolved boolean NOT NULL DEFAULT false,
    rating smallint,
    rated_at timestamptz,
    rated_member_id uuid,
    ai_involved boolean NOT NULL DEFAULT false,
    ai_answers integer NOT NULL DEFAULT 0,
    ai_first_answer_at timestamptz,
    ai_last_answer_at timestamptz,
    ai_handed_off_at timestamptz,
    ai_handoff_reason text,
    ai_confirmed boolean NOT NULL DEFAULT false,
    ai_cost_micros bigint NOT NULL DEFAULT 0,
    computed_at timestamptz NOT NULL
);

CREATE INDEX conversation_facts_workspace_id_started_at_index ON conversation_facts (workspace_id, started_at);
CREATE INDEX conversation_facts_workspace_id_closed_at_index ON conversation_facts (workspace_id, closed_at)
    WHERE closed_at IS NOT NULL;
CREATE INDEX conversation_facts_workspace_id_first_response_at_index
    ON conversation_facts (workspace_id, first_response_at) WHERE first_response_at IS NOT NULL;
CREATE INDEX conversation_facts_workspace_id_first_answer_at_index
    ON conversation_facts (workspace_id, first_answer_at) WHERE first_answer_at IS NOT NULL;
CREATE INDEX conversation_facts_workspace_id_first_closed_at_index
    ON conversation_facts (workspace_id, first_closed_at) WHERE first_closed_at IS NOT NULL;
CREATE INDEX conversation_facts_workspace_id_rated_at_index ON conversation_facts (workspace_id, rated_at)
    WHERE rated_at IS NOT NULL;
CREATE INDEX conversation_facts_workspace_id_open_index ON conversation_facts (workspace_id, started_at)
    WHERE status <> 'closed';

ALTER TABLE conversation_facts ENABLE ROW LEVEL SECURITY;
CREATE POLICY conversation_facts_tenant ON conversation_facts USING (workspace_id = ANY (dimmi_visible_workspaces()));

CREATE TABLE conversation_reply_facts (
    conversation_id uuid NOT NULL REFERENCES conversations ON DELETE CASCADE,
    turn integer NOT NULL,
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    asked_at timestamptz NOT NULL,
    replied_at timestamptz NOT NULL,
    member_id uuid NOT NULL,
    seconds bigint NOT NULL,
    business_seconds bigint NOT NULL,
    PRIMARY KEY (conversation_id, turn)
);

CREATE INDEX conversation_reply_facts_workspace_id_replied_at_index
    ON conversation_reply_facts (workspace_id, replied_at);

ALTER TABLE conversation_reply_facts ENABLE ROW LEVEL SECURITY;
CREATE POLICY conversation_reply_facts_tenant ON conversation_reply_facts
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

CREATE TABLE conversation_member_facts (
    conversation_id uuid NOT NULL REFERENCES conversations ON DELETE CASCADE,
    member_id uuid NOT NULL,
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    replies integer NOT NULL DEFAULT 0,
    first_replied_at timestamptz,
    last_replied_at timestamptz,
    assigned_open_seconds bigint NOT NULL DEFAULT 0,
    last_active_at timestamptz NOT NULL,
    PRIMARY KEY (conversation_id, member_id)
);

CREATE INDEX conversation_member_facts_workspace_id_last_replied_at_index
    ON conversation_member_facts (workspace_id, last_replied_at) WHERE last_replied_at IS NOT NULL;

ALTER TABLE conversation_member_facts ENABLE ROW LEVEL SECURITY;
CREATE POLICY conversation_member_facts_tenant ON conversation_member_facts
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

CREATE TABLE backlog_snapshots (
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    day date NOT NULL,
    inbox_id uuid NOT NULL,
    team_id uuid,
    open integer NOT NULL DEFAULT 0,
    snoozed integer NOT NULL DEFAULT 0,
    unassigned integer NOT NULL DEFAULT 0,
    under_day integer NOT NULL DEFAULT 0,
    under_three_days integer NOT NULL DEFAULT 0,
    under_week integer NOT NULL DEFAULT 0,
    over_week integer NOT NULL DEFAULT 0,
    taken_at timestamptz NOT NULL
);

CREATE UNIQUE INDEX backlog_snapshots_key ON backlog_snapshots (workspace_id, day, inbox_id, team_id) NULLS NOT DISTINCT;

ALTER TABLE backlog_snapshots ENABLE ROW LEVEL SECURITY;
CREATE POLICY backlog_snapshots_tenant ON backlog_snapshots USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- The workspaces whose day has ended since their last snapshot.
-- +goose StatementBegin
CREATE FUNCTION dimmi_report_snapshots_due(at timestamptz)
    RETURNS TABLE (workspace_id uuid, day date)
    LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        SELECT w.id, ((at AT TIME ZONE COALESCE(nullif(w.timezone, ''), 'UTC'))::date - 1)
        FROM workspaces w
        LEFT JOIN report_states s ON s.workspace_id = w.id
        WHERE w.deleted_at IS NULL
          AND (s.snapshot_day IS NULL
            OR s.snapshot_day < (at AT TIME ZONE COALESCE(nullif(w.timezone, ''), 'UTC'))::date - 1);
    $$;
-- +goose StatementEnd

-- +goose Down
DROP FUNCTION dimmi_report_snapshots_due(timestamptz);
DROP TABLE backlog_snapshots;
DROP TABLE conversation_member_facts;
DROP TABLE conversation_reply_facts;
DROP TABLE conversation_facts;
