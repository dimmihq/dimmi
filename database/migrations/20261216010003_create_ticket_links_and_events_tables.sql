-- migrate: phase=expand
-- Links tie tickets to other tickets and to conversations: a customer
-- ticket to the back-office work behind it, a tracker to every
-- conversation reporting the problem, an old ticket to the follow-up
-- opened after its reopen window, or any two related tickets.
-- A link points from one ticket to exactly one other ticket or one
-- conversation, once per kind.
--
-- Every change to a ticket is appended to its event log with who made
-- it: state, fields, type, links, issues, assignment and notes. The log
-- is the ticket's history, the source of time-in-state reports, and the
-- workspace's export cursor, so rows are never changed.

-- +goose Up
CREATE TABLE ticket_links (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    kind text NOT NULL CHECK (kind IN ('back_office', 'tracker', 'follow_up', 'related')),
    from_ticket_id uuid NOT NULL REFERENCES tickets ON DELETE CASCADE,
    to_ticket_id uuid REFERENCES tickets ON DELETE CASCADE,
    conversation_id uuid REFERENCES conversations ON DELETE CASCADE,
    created_by_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ticket_links_one_target_check CHECK ((to_ticket_id IS NULL) <> (conversation_id IS NULL))
);

CREATE UNIQUE INDEX ticket_links_kind_from_ticket_id_to_ticket_id_key ON ticket_links (kind, from_ticket_id, to_ticket_id)
    WHERE to_ticket_id IS NOT NULL;
CREATE UNIQUE INDEX ticket_links_kind_from_ticket_id_conversation_id_key
    ON ticket_links (kind, from_ticket_id, conversation_id) WHERE conversation_id IS NOT NULL;
CREATE INDEX ticket_links_to_ticket_id_index ON ticket_links (to_ticket_id);
CREATE INDEX ticket_links_conversation_id_index ON ticket_links (conversation_id);
CREATE INDEX ticket_links_created_by_member_id_index ON ticket_links (created_by_member_id);

ALTER TABLE ticket_links ENABLE ROW LEVEL SECURITY;
CREATE POLICY ticket_links_tenant ON ticket_links USING (workspace_id = ANY (dimmi_visible_workspaces()));

CREATE TABLE ticket_events (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    ticket_id uuid NOT NULL REFERENCES tickets ON DELETE CASCADE,
    kind text NOT NULL CHECK (kind ~ '^[a-z_]{1,40}$'),
    actor_type text NOT NULL DEFAULT 'system',
    actor_id uuid,
    data jsonb NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(data) = 'object'),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ticket_events_ticket_id_id_index ON ticket_events (ticket_id, id);
CREATE INDEX ticket_events_workspace_id_id_index ON ticket_events (workspace_id, id);

ALTER TABLE ticket_events ENABLE ROW LEVEL SECURITY;
CREATE POLICY ticket_events_tenant ON ticket_events USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- A company's monthly revenue is the number its custom attributes hold
-- under monthly_spend, or mrr, zero when neither is a number. Trackers
-- add it up as revenue at stake.
-- +goose StatementBegin
CREATE FUNCTION dimmi_company_revenue(attributes jsonb) RETURNS numeric
    LANGUAGE sql IMMUTABLE PARALLEL SAFE AS $$
        SELECT CASE
            WHEN jsonb_typeof(attributes -> 'monthly_spend') = 'number' THEN (attributes ->> 'monthly_spend')::numeric
            WHEN jsonb_typeof(attributes -> 'mrr') = 'number' THEN (attributes ->> 'mrr')::numeric
            ELSE 0 END
    $$;
-- +goose StatementEnd

-- +goose Down
DROP FUNCTION dimmi_company_revenue(jsonb);
DROP TABLE ticket_events;
DROP TABLE ticket_links;
