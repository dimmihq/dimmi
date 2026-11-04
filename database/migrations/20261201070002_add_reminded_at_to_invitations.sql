-- migrate: phase=expand
-- Whoever sends an invitation hears once when it has waited three days
-- without an answer, so a missed mail does not stall a new teammate. The
-- daily sweep looks across every workspace, which row level security
-- hides from the application role, so it reads and marks invitations
-- through security definer functions owned by the migration role. The
-- reader returns only what the reminder needs: pending, unexpired
-- invitations never reminded about whose sender is still an active
-- member of a live workspace. The marker touches only invitations not
-- reminded yet, so two sweeps never mail twice.

-- +goose Up
ALTER TABLE invitations ADD COLUMN reminded_at timestamptz;

CREATE FUNCTION auth_invitations_to_remind(sent_before timestamptz, at timestamptz, max_rows integer)
    RETURNS TABLE (
        id uuid, email citext, sent_at timestamptz, workspace_name text, inviter_name text,
        inviter_email citext, inviter_locale text
    )
    LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        SELECT i.id, i.email, i.last_sent_at, w.name, u.name, u.email, u.locale
        FROM invitations i
        JOIN workspace_members m ON m.id = i.invited_by_member_id AND m.workspace_id = i.workspace_id
        JOIN workspaces w ON w.id = i.workspace_id
        JOIN users u ON u.id = m.user_id
        WHERE i.status = 'pending' AND i.reminded_at IS NULL
          AND i.last_sent_at <= sent_before AND i.expires_at > at
          AND m.status = 'active' AND w.deleted_at IS NULL AND u.deleted_at IS NULL
        ORDER BY i.last_sent_at, i.id
        LIMIT max_rows
    $$;

CREATE FUNCTION auth_mark_invitation_reminded(invitation_id uuid, reminded timestamptz)
    RETURNS bigint
    LANGUAGE sql VOLATILE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        WITH marked AS (
            UPDATE invitations
            SET reminded_at = reminded, updated_at = reminded
            WHERE id = invitation_id AND reminded_at IS NULL AND status = 'pending'
            RETURNING id
        )
        SELECT count(*) FROM marked
    $$;

-- +goose Down
DROP FUNCTION auth_mark_invitation_reminded(uuid, timestamptz);
DROP FUNCTION auth_invitations_to_remind(timestamptz, timestamptz, integer);
ALTER TABLE invitations DROP COLUMN reminded_at;
