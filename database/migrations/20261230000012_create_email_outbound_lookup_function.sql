-- migrate: phase=expand
-- A sending provider's delivery event names only the id it gave the
-- email, so the workspace that sent it is found before any workspace is
-- set, while row level security would hide every sent email. This
-- lookup runs as a security definer function owned by the migration
-- role and returns only whose email it is.

-- +goose Up
CREATE FUNCTION email_outbound_workspace(sent_transport text, sent_provider_message_id text)
    RETURNS TABLE (workspace_id uuid)
    LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        SELECT o.workspace_id
        FROM outbound_emails o
        WHERE o.transport = sent_transport AND o.provider_message_id = sent_provider_message_id
    $$;

-- +goose Down
DROP FUNCTION email_outbound_workspace(text, text);
