-- migrate: phase=expand
-- One email addressed to two workspaces is stored once and recorded by
-- both. Before a workspace's retention, an erasure or a purge deletes
-- the stored source, it asks whether another record still points at the
-- file, across every workspace, which row level security hides from the
-- application role; the answer says nothing else about the other record.

-- +goose NO TRANSACTION
-- +goose Up
CREATE FUNCTION dimmi_raw_email_shared(stored_disk text, stored_path text, except_ids uuid[])
    RETURNS boolean
    LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        SELECT EXISTS (
            SELECT 1 FROM inbound_emails
            WHERE raw_path = stored_path AND raw_disk = stored_disk AND NOT (id = ANY (except_ids))
        )
    $$;

CREATE INDEX CONCURRENTLY inbound_emails_raw_path_index ON inbound_emails (raw_path) WHERE raw_path IS NOT NULL;

-- +goose Down
DROP INDEX CONCURRENTLY IF EXISTS inbound_emails_raw_path_index;
DROP FUNCTION dimmi_raw_email_shared(text, text, uuid[]);
