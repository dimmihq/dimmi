-- migrate: phase=expand
-- Service level policies set targets the team promises: the first reply,
-- every next reply, a follow-up when the team spoke last and nothing
-- happened since, and resolution. A policy counts time in an office
-- hours schedule when it names one, warns some time before a target is
-- due, and pauses while the work waits on the customer. On a breach it
-- may raise the priority or move the work to a team.
--
-- A clock is one target running on one ticket or one conversation. It
-- runs, pauses, and ends met, breached or canceled. Due and warn times
-- are kept on the row so a sweep finds what to warn about and what has
-- breached across all workspaces through definer functions.

-- +goose Up
CREATE TABLE sla_policies (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
    description text NOT NULL DEFAULT '' CHECK (char_length(description) <= 500),
    schedule_id uuid REFERENCES schedules ON DELETE SET NULL,
    first_reply_seconds integer CHECK (first_reply_seconds > 0),
    next_reply_seconds integer CHECK (next_reply_seconds > 0),
    follow_up_seconds integer CHECK (follow_up_seconds > 0),
    resolution_seconds integer CHECK (resolution_seconds > 0),
    warn_before_seconds integer NOT NULL DEFAULT 900 CHECK (warn_before_seconds >= 0),
    pause_on_waiting_customer boolean NOT NULL DEFAULT true,
    pause_on_waiting_team boolean NOT NULL DEFAULT false,
    breach_priority text CHECK (breach_priority IN ('low', 'medium', 'high', 'urgent')),
    breach_team_id uuid REFERENCES teams ON DELETE SET NULL,
    ticket_type_ids uuid[] NOT NULL DEFAULT '{}',
    priorities text[] NOT NULL DEFAULT '{}',
    position integer NOT NULL DEFAULT 0,
    archived_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX sla_policies_workspace_id_position_index ON sla_policies (workspace_id, position);
CREATE INDEX sla_policies_schedule_id_index ON sla_policies (schedule_id);
CREATE INDEX sla_policies_breach_team_id_index ON sla_policies (breach_team_id);

ALTER TABLE sla_policies ENABLE ROW LEVEL SECURITY;
CREATE POLICY sla_policies_tenant ON sla_policies USING (workspace_id = ANY (dimmi_visible_workspaces()));

ALTER TABLE tickets ADD CONSTRAINT tickets_sla_policy_id_fkey
    FOREIGN KEY (sla_policy_id) REFERENCES sla_policies ON DELETE SET NULL;

CREATE TABLE sla_clocks (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    policy_id uuid NOT NULL REFERENCES sla_policies ON DELETE CASCADE,
    ticket_id uuid REFERENCES tickets ON DELETE CASCADE,
    conversation_id uuid REFERENCES conversations ON DELETE CASCADE,
    target text NOT NULL CHECK (target IN ('first_reply', 'next_reply', 'follow_up', 'resolution')),
    state text NOT NULL DEFAULT 'running' CHECK (state IN ('running', 'paused', 'met', 'breached', 'canceled')),
    started_at timestamptz NOT NULL,
    due_at timestamptz NOT NULL,
    warn_at timestamptz,
    paused_at timestamptz,
    warned_at timestamptz,
    breached_at timestamptz,
    stopped_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT sla_clocks_one_subject_check CHECK ((ticket_id IS NULL) <> (conversation_id IS NULL))
);

CREATE UNIQUE INDEX sla_clocks_ticket_id_target_key ON sla_clocks (ticket_id, target)
    WHERE ticket_id IS NOT NULL AND state IN ('running', 'paused');
CREATE UNIQUE INDEX sla_clocks_conversation_id_target_key ON sla_clocks (conversation_id, target)
    WHERE conversation_id IS NOT NULL AND state IN ('running', 'paused');
CREATE INDEX sla_clocks_ticket_id_index ON sla_clocks (ticket_id);
CREATE INDEX sla_clocks_conversation_id_index ON sla_clocks (conversation_id);
CREATE INDEX sla_clocks_policy_id_index ON sla_clocks (policy_id);
CREATE INDEX sla_clocks_running_due_at_index ON sla_clocks (due_at) WHERE state = 'running';
CREATE INDEX sla_clocks_running_warn_at_index ON sla_clocks (warn_at)
    WHERE state = 'running' AND warned_at IS NULL AND warn_at IS NOT NULL;
CREATE INDEX sla_clocks_workspace_id_created_at_index ON sla_clocks (workspace_id, created_at DESC);

ALTER TABLE sla_clocks ENABLE ROW LEVEL SECURITY;
CREATE POLICY sla_clocks_tenant ON sla_clocks USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose StatementBegin
CREATE FUNCTION sla_clocks_due(due_at timestamptz, row_limit integer)
    RETURNS TABLE (workspace_id uuid, id uuid)
    LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        (SELECT c.workspace_id, c.id FROM sla_clocks c
         WHERE c.state = 'running' AND c.due_at <= sla_clocks_due.due_at
         ORDER BY c.due_at LIMIT row_limit)
        UNION
        (SELECT c.workspace_id, c.id FROM sla_clocks c
         WHERE c.state = 'running' AND c.warned_at IS NULL AND c.warn_at IS NOT NULL
           AND c.warn_at <= sla_clocks_due.due_at
         ORDER BY c.warn_at LIMIT row_limit)
    $$;
-- +goose StatementEnd

-- +goose Down
DROP FUNCTION sla_clocks_due(timestamptz, integer);
DROP TABLE sla_clocks;
ALTER TABLE tickets DROP CONSTRAINT tickets_sla_policy_id_fkey;
DROP TABLE sla_policies;
