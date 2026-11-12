-- migrate: phase=expand
-- Each teammate notification on each channel leaves one row: sent,
-- failed and why, or skipped and why, so a teammate can see why they did
-- or did not hear about something. A delivery retried after a failure
-- updates its row. A Slack channel post names its destination instead of
-- a member; destinations come later in the migration order, so that
-- column has no foreign key. Rows held back by quiet hours wait for the
-- summary sent when the quiet ends, which stamps them. Rows are kept for
-- thirty days; the functions let the nightly prune and the summary sweep
-- find their rows across workspaces.

-- +goose Up
CREATE TABLE notification_deliveries (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    workspace_member_id uuid REFERENCES workspace_members ON DELETE CASCADE,
    notification_destination_id uuid,
    delivery_id uuid NOT NULL,
    trigger text NOT NULL CHECK (trigger ~ '^[a-z][a-z0-9_]{1,39}$'),
    channel text NOT NULL CHECK (channel IN ('in_app', 'email', 'push', 'slack')),
    status text NOT NULL CHECK (status IN ('sent', 'failed', 'skipped')),
    reason text,
    detail text,
    title text NOT NULL DEFAULT '',
    link text NOT NULL DEFAULT '',
    attempts integer NOT NULL DEFAULT 1,
    occurred_at timestamptz NOT NULL,
    summarized_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (delivery_id, channel)
);

CREATE INDEX notification_deliveries_member_index
    ON notification_deliveries (workspace_id, workspace_member_id, created_at DESC, id DESC);
CREATE INDEX notification_deliveries_workspace_index
    ON notification_deliveries (workspace_id, created_at DESC, id DESC);
CREATE INDEX notification_deliveries_created_at_index ON notification_deliveries (created_at);
CREATE INDEX notification_deliveries_quiet_index
    ON notification_deliveries (workspace_id, workspace_member_id)
    WHERE reason = 'quiet_hours' AND summarized_at IS NULL;

ALTER TABLE notification_deliveries ENABLE ROW LEVEL SECURITY;
CREATE POLICY notification_deliveries_tenant ON notification_deliveries
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

CREATE FUNCTION prune_notification_deliveries(before timestamptz, max_rows integer)
    RETURNS bigint
    LANGUAGE sql VOLATILE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        WITH gone AS (
            DELETE FROM notification_deliveries
            WHERE id IN (SELECT id FROM notification_deliveries WHERE created_at < before LIMIT max_rows)
            RETURNING 1
        )
        SELECT count(*) FROM gone
    $$;

CREATE FUNCTION notification_quiet_held(max_rows integer)
    RETURNS TABLE (workspace_id uuid, workspace_member_id uuid)
    LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        SELECT DISTINCT d.workspace_id, d.workspace_member_id
        FROM notification_deliveries d
        WHERE d.reason = 'quiet_hours' AND d.summarized_at IS NULL AND d.workspace_member_id IS NOT NULL
        LIMIT max_rows
    $$;

-- +goose Down
DROP FUNCTION notification_quiet_held(integer);
DROP FUNCTION prune_notification_deliveries(timestamptz, integer);
DROP TABLE notification_deliveries;
