-- migrate: phase=expand
-- How readers use a help center, kept as counts per day and nothing
-- about who read: article_views counts the times an article was opened
-- on the public pages or in the messenger, and help_search_terms counts
-- each search's words and how often they found nothing, which is where
-- articles are missing. Rows are added to in place, one per article,
-- day and source, or per help center, day and words. Old days are
-- pruned every night.

-- +goose Up
CREATE TABLE article_views (
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    help_center_id uuid NOT NULL REFERENCES help_centers ON DELETE CASCADE,
    article_id uuid NOT NULL REFERENCES articles ON DELETE CASCADE,
    day date NOT NULL,
    source text NOT NULL CHECK (source IN ('web', 'messenger')),
    views integer NOT NULL DEFAULT 0,
    PRIMARY KEY (article_id, day, source)
);

CREATE INDEX article_views_help_center_id_day_index ON article_views (help_center_id, day);

ALTER TABLE article_views ENABLE ROW LEVEL SECURITY;
CREATE POLICY article_views_tenant ON article_views USING (workspace_id = ANY (dimmi_visible_workspaces()));

CREATE TABLE help_search_terms (
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    help_center_id uuid NOT NULL REFERENCES help_centers ON DELETE CASCADE,
    day date NOT NULL,
    term text NOT NULL CHECK (char_length(term) BETWEEN 1 AND 100),
    searches integer NOT NULL DEFAULT 0,
    misses integer NOT NULL DEFAULT 0,
    PRIMARY KEY (help_center_id, day, term)
);

ALTER TABLE help_search_terms ENABLE ROW LEVEL SECURITY;
CREATE POLICY help_search_terms_tenant ON help_search_terms
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose StatementBegin
CREATE FUNCTION help_analytics_prune(views_before timestamptz, terms_before timestamptz) RETURNS void
    LANGUAGE sql VOLATILE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        DELETE FROM article_views WHERE day < views_before::date;
        DELETE FROM help_search_terms WHERE day < terms_before::date;
    $$;
-- +goose StatementEnd

-- +goose Down
DROP FUNCTION help_analytics_prune(timestamptz, timestamptz);
DROP TABLE help_search_terms;
DROP TABLE article_views;
