-- migrate: phase=expand
-- How a workspace sets up tickets. States are workspace-wide, each in one
-- of five fixed groups that say what it means: submitted, in progress,
-- waiting on the customer, waiting on the team, resolved. A state has
-- the name the team sees, the label the customer sees, and whether
-- moving into it tells the customer. A purpose marks the few states the
-- product itself moves tickets to, such as "close the loop".
--
-- A type is a kind of ticket: a customer's request, back-office work,
-- a tracker that gathers many reports of one problem, or an incident.
-- It names its default inbox and team, the states it may use, and for
-- customer types whether the messenger and portal show it, whether the
-- requester's company sees it, how long a resolved ticket may reopen,
-- and when a ticket waiting on the customer resolves itself. Fields are
-- per type, typed like attributes, each saying when it is required and
-- what the customer sees of it. Archiving hides a type, field or state
-- from new work and keeps it on old tickets.

-- +goose Up
CREATE TABLE ticket_states (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    state_group text NOT NULL
        CHECK (state_group IN ('submitted', 'in_progress', 'waiting_customer', 'waiting_team', 'resolved')),
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 60),
    customer_label text NOT NULL CHECK (char_length(customer_label) BETWEEN 1 AND 80),
    notify_customer boolean NOT NULL DEFAULT true,
    purpose text CHECK (purpose ~ '^[a-z_]{1,40}$'),
    position integer NOT NULL DEFAULT 0,
    archived_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ticket_states_workspace_id_position_index ON ticket_states (workspace_id, position);
CREATE UNIQUE INDEX ticket_states_workspace_id_purpose_key ON ticket_states (workspace_id, purpose)
    WHERE purpose IS NOT NULL AND archived_at IS NULL;

ALTER TABLE ticket_states ENABLE ROW LEVEL SECURITY;
CREATE POLICY ticket_states_tenant ON ticket_states USING (workspace_id = ANY (dimmi_visible_workspaces()));

CREATE TABLE ticket_types (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    kind text NOT NULL CHECK (kind IN ('customer', 'back_office', 'tracker', 'incident')),
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 60),
    icon text NOT NULL DEFAULT 'ticket' CHECK (icon ~ '^[a-z0-9-]{1,40}$'),
    description text NOT NULL DEFAULT '' CHECK (char_length(description) <= 500),
    inbox_id uuid REFERENCES inboxes ON DELETE SET NULL,
    team_id uuid REFERENCES teams ON DELETE SET NULL,
    customer_visible boolean NOT NULL DEFAULT false,
    company_sharing boolean NOT NULL DEFAULT false,
    reopen_days integer NOT NULL DEFAULT 7 CHECK (reopen_days BETWEEN 0 AND 90),
    auto_resolve_days integer CHECK (auto_resolve_days BETWEEN 1 AND 90),
    auto_resolve_message text CHECK (char_length(auto_resolve_message) <= 2000),
    ai_auto_fill boolean NOT NULL DEFAULT false,
    position integer NOT NULL DEFAULT 0,
    archived_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ticket_types_workspace_id_position_index ON ticket_types (workspace_id, position);
CREATE INDEX ticket_types_inbox_id_index ON ticket_types (inbox_id);
CREATE INDEX ticket_types_team_id_index ON ticket_types (team_id);

ALTER TABLE ticket_types ENABLE ROW LEVEL SECURITY;
CREATE POLICY ticket_types_tenant ON ticket_types USING (workspace_id = ANY (dimmi_visible_workspaces()));

CREATE TABLE ticket_type_states (
    type_id uuid NOT NULL REFERENCES ticket_types ON DELETE CASCADE,
    state_id uuid NOT NULL REFERENCES ticket_states ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    is_default boolean NOT NULL DEFAULT false,
    is_default_resolved boolean NOT NULL DEFAULT false,
    PRIMARY KEY (type_id, state_id)
);

CREATE INDEX ticket_type_states_state_id_index ON ticket_type_states (state_id);

ALTER TABLE ticket_type_states ENABLE ROW LEVEL SECURITY;
CREATE POLICY ticket_type_states_tenant ON ticket_type_states
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

CREATE TABLE ticket_fields (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    type_id uuid NOT NULL REFERENCES ticket_types ON DELETE CASCADE,
    key text NOT NULL CHECK (key ~ '^[a-z][a-z0-9_]{0,39}$'),
    label text NOT NULL CHECK (char_length(label) BETWEEN 1 AND 80),
    description text NOT NULL DEFAULT '' CHECK (char_length(description) <= 300),
    data_type text NOT NULL CHECK (data_type IN (
        'short_text', 'long_text', 'number', 'decimal', 'currency', 'boolean', 'date', 'single_choice',
        'multiple_choice', 'file', 'member', 'company'
    )),
    options text[] NOT NULL DEFAULT '{}' CHECK (cardinality(options) <= 200),
    currency text CHECK (currency ~ '^[A-Z]{3}$'),
    required_create boolean NOT NULL DEFAULT false,
    required_create_customer boolean NOT NULL DEFAULT false,
    required_resolve boolean NOT NULL DEFAULT false,
    visible_customer boolean NOT NULL DEFAULT false,
    editable_customer boolean NOT NULL DEFAULT false,
    show_when jsonb CHECK (show_when IS NULL OR jsonb_typeof(show_when) = 'object'),
    ai_hint text CHECK (char_length(ai_hint) <= 500),
    ai_auto boolean NOT NULL DEFAULT false,
    position integer NOT NULL DEFAULT 0,
    archived_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX ticket_fields_type_id_key_key ON ticket_fields (type_id, key);

ALTER TABLE ticket_fields ENABLE ROW LEVEL SECURITY;
CREATE POLICY ticket_fields_tenant ON ticket_fields USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE ticket_fields;
DROP TABLE ticket_type_states;
DROP TABLE ticket_types;
DROP TABLE ticket_states;
