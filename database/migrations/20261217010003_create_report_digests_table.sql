-- migrate: phase=expand
-- A teammate can have the reports mailed to them: a daily digest of the
-- day before, or a weekly one of the week before, sent in the morning of
-- their own time zone. One subscription per teammate; next_send_at says
-- when the next one goes, and the send moves it on.

-- +goose Up
CREATE TABLE report_digests (
    id uuid PRIMARY KEY,
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    member_id uuid NOT NULL REFERENCES workspace_members ON DELETE CASCADE,
    frequency text NOT NULL CHECK (frequency IN ('daily', 'weekly')),
    filters jsonb NOT NULL DEFAULT '{}',
    next_send_at timestamptz NOT NULL,
    last_sent_at timestamptz,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);

CREATE UNIQUE INDEX report_digests_member_id_key ON report_digests (workspace_id, member_id);
CREATE INDEX report_digests_next_send_at_index ON report_digests (next_send_at);

ALTER TABLE report_digests ENABLE ROW LEVEL SECURITY;
CREATE POLICY report_digests_tenant ON report_digests USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- The digests due to go out, across workspaces.
-- +goose StatementBegin
CREATE FUNCTION dimmi_report_digests_due(at timestamptz, batch integer)
    RETURNS TABLE (workspace_id uuid, id uuid)
    LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        SELECT d.workspace_id, d.id FROM report_digests d
        WHERE d.next_send_at <= at
        ORDER BY d.next_send_at
        LIMIT batch;
    $$;
-- +goose StatementEnd

-- +goose Down
DROP FUNCTION dimmi_report_digests_due(timestamptz, integer);
DROP TABLE report_digests;
