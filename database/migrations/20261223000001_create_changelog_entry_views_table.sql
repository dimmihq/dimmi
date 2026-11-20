-- migrate: phase=expand
-- How readers use a changelog, kept as counts per day and nothing about
-- who read: changelog_entry_views counts the times an update's page was
-- opened. Rows are added to in place, one per update and day. Old days
-- are pruned every night with the help center's other counts.

-- +goose Up
CREATE TABLE changelog_entry_views (
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    help_center_id uuid NOT NULL REFERENCES help_centers ON DELETE CASCADE,
    entry_id uuid NOT NULL REFERENCES changelog_entries ON DELETE CASCADE,
    day date NOT NULL,
    views integer NOT NULL DEFAULT 0,
    PRIMARY KEY (entry_id, day)
);

CREATE INDEX changelog_entry_views_help_center_id_day_index ON changelog_entry_views (help_center_id, day);

ALTER TABLE changelog_entry_views ENABLE ROW LEVEL SECURITY;
CREATE POLICY changelog_entry_views_tenant ON changelog_entry_views
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION help_analytics_prune(views_before timestamptz, terms_before timestamptz) RETURNS void
    LANGUAGE sql VOLATILE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        DELETE FROM article_views WHERE day < views_before::date;
        DELETE FROM changelog_entry_views WHERE day < views_before::date;
        DELETE FROM help_search_terms WHERE day < terms_before::date;
    $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION help_analytics_prune(views_before timestamptz, terms_before timestamptz) RETURNS void
    LANGUAGE sql VOLATILE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        DELETE FROM article_views WHERE day < views_before::date;
        DELETE FROM help_search_terms WHERE day < terms_before::date;
    $$;
-- +goose StatementEnd
DROP TABLE changelog_entry_views;
