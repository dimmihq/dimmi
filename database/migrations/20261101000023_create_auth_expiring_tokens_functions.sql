-- migrate: phase=expand
-- Owners hear a week ahead that a token of theirs is about to expire.
-- The daily sweep looks across every workspace at once, which row level
-- security hides from the application role until a workspace is set, so
-- it reads and marks tokens through security definer functions owned by
-- the migration role. The reader returns only what the warning needs:
-- live tokens of active members that were not warned yet. The marker
-- touches only tokens still unwarned, so two sweeps never mail twice.

-- +goose Up
CREATE FUNCTION auth_tokens_expiring(expiring_after timestamptz, expiring_before timestamptz, max_rows integer)
    RETURNS TABLE (
        id uuid, name text, expires_at timestamptz, workspace_name text, user_name text, user_email citext,
        user_locale text
    )
    LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        SELECT t.id, t.name, t.expires_at, w.name, u.name, u.email, u.locale
        FROM personal_access_tokens t
        JOIN workspace_members m ON m.id = t.owner_id AND m.workspace_id = t.workspace_id
        JOIN workspaces w ON w.id = t.workspace_id
        JOIN users u ON u.id = m.user_id
        WHERE t.revoked_at IS NULL AND t.expiry_notified_at IS NULL
          AND t.expires_at > expiring_after AND t.expires_at <= expiring_before
          AND m.status = 'active' AND w.deleted_at IS NULL AND u.deleted_at IS NULL
        ORDER BY t.expires_at, t.id
        LIMIT max_rows
    $$;

CREATE FUNCTION auth_mark_token_expiry_notified(token_id uuid, notified timestamptz)
    RETURNS bigint
    LANGUAGE sql VOLATILE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        WITH marked AS (
            UPDATE personal_access_tokens
            SET expiry_notified_at = notified, updated_at = notified
            WHERE id = token_id AND expiry_notified_at IS NULL
            RETURNING id
        )
        SELECT count(*) FROM marked
    $$;

-- +goose Down
DROP FUNCTION auth_mark_token_expiry_notified(uuid, timestamptz);
DROP FUNCTION auth_tokens_expiring(timestamptz, timestamptz, integer);
