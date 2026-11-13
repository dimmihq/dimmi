-- migrate: phase=expand
-- A ticket is tracked work with a type, a state and field values. Its
-- number is the human reference, T-482, handed out per workspace from
-- workspaces.next_ticket_number in the same transaction. A customer
-- ticket is tied to one conversation, where its replies live; other
-- kinds have none. The state's group is copied onto the row so queues,
-- counts and reports read one column. Version goes up on every change,
-- so an edit made against an older copy is refused. External id lets a
-- client's retry find the ticket it already made. Field values the AI
-- proposed wait in suggestions until a person accepts them.
--
-- Inboxes may turn every new conversation into a ticket of a type.

-- +goose Up
ALTER TABLE workspaces ADD COLUMN next_ticket_number bigint NOT NULL DEFAULT 1;

CREATE TABLE tickets (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    number bigint NOT NULL,
    kind text NOT NULL CHECK (kind IN ('customer', 'back_office', 'tracker')),
    type_id uuid NOT NULL REFERENCES ticket_types,
    state_id uuid NOT NULL REFERENCES ticket_states,
    state_group text NOT NULL
        CHECK (state_group IN ('submitted', 'in_progress', 'waiting_customer', 'waiting_team', 'resolved')),
    conversation_id uuid REFERENCES conversations ON DELETE SET NULL,
    contact_id uuid REFERENCES contacts ON DELETE SET NULL,
    company_id uuid REFERENCES companies ON DELETE SET NULL,
    inbox_id uuid NOT NULL REFERENCES inboxes,
    title text NOT NULL CHECK (char_length(title) BETWEEN 1 AND 200),
    description text NOT NULL DEFAULT '' CHECK (char_length(description) <= 20000),
    fields jsonb NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(fields) = 'object'),
    suggestions jsonb NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(suggestions) = 'object'),
    priority text NOT NULL DEFAULT 'none' CHECK (priority IN ('none', 'low', 'medium', 'high', 'urgent')),
    assignee_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    assignee_team_id uuid REFERENCES teams ON DELETE SET NULL,
    external_id text CHECK (char_length(external_id) BETWEEN 1 AND 255),
    sla_policy_id uuid,
    close_loop boolean NOT NULL DEFAULT false,
    notified_state_id uuid REFERENCES ticket_states ON DELETE SET NULL,
    customer_visible boolean NOT NULL DEFAULT true,
    version integer NOT NULL DEFAULT 1,
    created_by_type text NOT NULL DEFAULT 'system'
        CHECK (created_by_type IN ('member', 'contact', 'agent', 'workflow', 'integration', 'system')),
    created_by_id uuid,
    state_since timestamptz NOT NULL DEFAULT now(),
    resolved_at timestamptz,
    auto_resolve_warned_at timestamptz,
    deleted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT tickets_workspace_id_number_key UNIQUE (workspace_id, number)
);

CREATE UNIQUE INDEX tickets_conversation_id_key ON tickets (conversation_id) WHERE conversation_id IS NOT NULL;
CREATE UNIQUE INDEX tickets_workspace_id_external_id_key ON tickets (workspace_id, external_id)
    WHERE external_id IS NOT NULL;
CREATE INDEX tickets_workspace_id_created_at_id_index ON tickets (workspace_id, created_at DESC, id DESC);
CREATE INDEX tickets_workspace_id_type_id_state_group_index ON tickets (workspace_id, type_id, state_group);
CREATE INDEX tickets_workspace_id_kind_state_group_index ON tickets (workspace_id, kind, state_group);
CREATE INDEX tickets_workspace_id_assignee_member_id_index ON tickets (workspace_id, assignee_member_id);
CREATE INDEX tickets_state_id_index ON tickets (state_id);
CREATE INDEX tickets_notified_state_id_index ON tickets (notified_state_id);
CREATE INDEX tickets_type_id_index ON tickets (type_id);
CREATE INDEX tickets_contact_id_index ON tickets (contact_id);
CREATE INDEX tickets_company_id_index ON tickets (company_id);
CREATE INDEX tickets_inbox_id_index ON tickets (inbox_id);
CREATE INDEX tickets_sla_policy_id_index ON tickets (sla_policy_id);
CREATE INDEX tickets_assignee_team_id_index ON tickets (assignee_team_id);
CREATE INDEX tickets_fields_index ON tickets USING gin (fields jsonb_path_ops);

ALTER TABLE tickets ENABLE ROW LEVEL SECURITY;
CREATE POLICY tickets_tenant ON tickets USING (workspace_id = ANY (dimmi_visible_workspaces()));

ALTER TABLE inboxes ADD COLUMN ticket_type_id uuid REFERENCES ticket_types ON DELETE SET NULL;

-- +goose StatementBegin
CREATE FUNCTION tickets_due_auto_resolve(due_at timestamptz, row_limit integer)
    RETURNS TABLE (workspace_id uuid, id uuid)
    LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        SELECT t.workspace_id, t.id
        FROM tickets t
        JOIN ticket_types ty ON ty.id = t.type_id
        WHERE t.state_group = 'waiting_customer' AND t.deleted_at IS NULL AND ty.auto_resolve_days IS NOT NULL
          AND t.state_since + make_interval(days => ty.auto_resolve_days - 1) <= due_at
        ORDER BY t.state_since
        LIMIT row_limit
    $$;
-- +goose StatementEnd

-- +goose Down
DROP FUNCTION tickets_due_auto_resolve(timestamptz, integer);
ALTER TABLE inboxes DROP COLUMN ticket_type_id;
DROP TABLE tickets;
ALTER TABLE workspaces DROP COLUMN next_ticket_number;
