-- migrate: phase=expand
-- Linked issues keep the provider's team id, when the issue was seen
-- done, when its fix was seen deployed, its severity and a fingerprint
-- of the impact last written back to it, so the impact is only written
-- again when it changed.
--
-- A provider delivers to an installation's own address before any
-- workspace is known, so a security definer function names the
-- workspace and provider of an installation, and nothing else.

-- +goose Up
ALTER TABLE ticket_issues ADD COLUMN team_id text NOT NULL DEFAULT '' CHECK (char_length(team_id) <= 200);
ALTER TABLE ticket_issues ADD COLUMN severity text NOT NULL DEFAULT '' CHECK (char_length(severity) <= 100);
ALTER TABLE ticket_issues ADD COLUMN done_at timestamptz;
ALTER TABLE ticket_issues ADD COLUMN deployed_at timestamptz;
ALTER TABLE ticket_issues ADD COLUMN impact_hash text NOT NULL DEFAULT '' CHECK (char_length(impact_hash) <= 64);

-- +goose StatementBegin
CREATE FUNCTION integration_installation_owner(installation_id uuid)
    RETURNS TABLE (workspace_id uuid, provider text, status text)
    LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        SELECT i.workspace_id, i.provider, i.status FROM integration_installations i
        WHERE i.id = installation_id AND i.deleted_at IS NULL
    $$;
-- +goose StatementEnd

-- +goose Down
DROP FUNCTION integration_installation_owner(uuid);
ALTER TABLE ticket_issues DROP COLUMN impact_hash;
ALTER TABLE ticket_issues DROP COLUMN deployed_at;
ALTER TABLE ticket_issues DROP COLUMN done_at;
ALTER TABLE ticket_issues DROP COLUMN severity;
ALTER TABLE ticket_issues DROP COLUMN team_id;
