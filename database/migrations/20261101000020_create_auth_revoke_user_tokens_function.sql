-- migrate: phase=expand
-- Some changes end every access token a user holds, whatever workspace
-- each one is bound to, such as when the owner of an inbox takes back an
-- account a stranger registered with it first. Tokens are tenant rows,
-- hidden from the application role until a workspace is set, so the
-- revocation runs as a security definer function owned by the migration
-- role. It touches only the user's own live tokens, pins search_path,
-- and returns how many it revoked.

-- +goose Up
CREATE FUNCTION auth_revoke_user_tokens(token_user_id uuid, revoked timestamptz)
    RETURNS bigint
    LANGUAGE sql VOLATILE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        WITH ended AS (
            UPDATE personal_access_tokens t
            SET revoked_at = revoked, updated_at = revoked
            FROM workspace_members m
            WHERE m.id = t.owner_id AND m.workspace_id = t.workspace_id AND m.user_id = token_user_id
              AND t.revoked_at IS NULL
            RETURNING t.id
        )
        SELECT count(*) FROM ended
    $$;

-- +goose Down
DROP FUNCTION auth_revoke_user_tokens(uuid, timestamptz);
