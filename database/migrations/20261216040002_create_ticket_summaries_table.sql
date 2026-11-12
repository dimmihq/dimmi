-- migrate: phase=expand
-- A tracker's summary says what its customers report, in a few
-- sentences the cheap model writes from their first words. It is kept
-- with the number of reports it read, and written again only once more
-- reports joined.

-- +goose Up
CREATE TABLE ticket_summaries (
    ticket_id uuid PRIMARY KEY REFERENCES tickets ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    summary text NOT NULL DEFAULT '' CHECK (char_length(summary) <= 4000),
    report_count integer NOT NULL DEFAULT 0,
    generated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ticket_summaries_workspace_id_index ON ticket_summaries (workspace_id);

ALTER TABLE ticket_summaries ENABLE ROW LEVEL SECURITY;
CREATE POLICY ticket_summaries_tenant ON ticket_summaries USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE ticket_summaries;
