-- migrate: phase=expand
-- Events are what customers did in the team's product: "exported a
-- report", "invited a teammate". No raw event is kept. Each person keeps
-- one summary per event: how many times, the first and last time, a
-- count for each of the last 90 days and the last event's details, which
-- is all a rule like "did X 3 times in 7 days" needs. daily_day is the UTC
-- midnight of the day daily[1] counts; daily[2] counts the day before,
-- and so on.
--
-- A definition is made by the team, or observed on the first trusted
-- event of a name. Archived names take no new events.

-- +goose Up
CREATE TABLE event_definitions (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    name text NOT NULL CHECK (name ~ '^[a-z0-9_.:-]{1,64}$'),
    label text NOT NULL CHECK (char_length(label) BETWEEN 1 AND 80),
    description text CHECK (char_length(description) <= 500),
    origin text NOT NULL DEFAULT 'created' CHECK (origin IN ('created', 'observed')),
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'archived')),
    last_seen_at timestamptz,
    total_count bigint NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX event_definitions_workspace_id_name_key ON event_definitions (workspace_id, name);

ALTER TABLE event_definitions ENABLE ROW LEVEL SECURITY;
CREATE POLICY event_definitions_tenant ON event_definitions USING (workspace_id = ANY (dimmi_visible_workspaces()));

CREATE TABLE contact_event_summaries (
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    contact_id uuid NOT NULL REFERENCES contacts ON DELETE CASCADE,
    event_definition_id uuid NOT NULL REFERENCES event_definitions ON DELETE CASCADE,
    count bigint NOT NULL,
    first_at timestamptz NOT NULL,
    last_at timestamptz NOT NULL,
    daily_day timestamptz NOT NULL,
    daily smallint[] NOT NULL,
    last_metadata jsonb NOT NULL DEFAULT '{}' CHECK (octet_length(last_metadata::text) <= 2048),
    updated_at timestamptz NOT NULL,
    PRIMARY KEY (workspace_id, contact_id, event_definition_id)
);

CREATE INDEX contact_event_summaries_workspace_id_event_definition_id_last_at_index
    ON contact_event_summaries (workspace_id, event_definition_id, last_at);

ALTER TABLE contact_event_summaries ENABLE ROW LEVEL SECURITY;
CREATE POLICY contact_event_summaries_tenant ON contact_event_summaries
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE contact_event_summaries;
DROP TABLE event_definitions;
