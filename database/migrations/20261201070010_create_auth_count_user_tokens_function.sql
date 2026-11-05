-- migrate: phase=expand
-- Changing a password leaves access tokens working, so the person is told how
-- many still do, whatever workspace each is bound to. Tokens are tenant rows,
-- hidden from the application role until a workspace is set, so the count runs
-- as a security definer function owned by the migration role. It reads only
-- the user's own tokens, pins search_path, and returns a number, never a row.

-- +goose Up
CREATE FUNCTION auth_count_user_tokens(token_user_id uuid, at timestamptz)
    RETURNS bigint
    LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        SELECT count(*)
        FROM personal_access_tokens t
        JOIN workspace_members m ON m.id = t.owner_id AND m.workspace_id = t.workspace_id
        WHERE m.user_id = token_user_id AND t.revoked_at IS NULL AND (t.expires_at IS NULL OR t.expires_at > at)
    $$;

-- +goose Down
DROP FUNCTION auth_count_user_tokens(uuid, timestamptz);
