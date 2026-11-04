-- name: ReplaceWidgetContactEmailIdentity :exec
-- ReplaceWidgetContactEmailIdentity drops the unproven address a lead typed
-- before and records the new one, unless another contact already holds that
-- address, which a lead's typing never takes from them.
WITH dropped AS (
    DELETE FROM contact_identities
    WHERE workspace_id = @workspace_id AND contact_id = @contact_id AND type = 'email' AND verified = false
)
INSERT INTO contact_identities (workspace_id, contact_id, type, value, verified, created_at, updated_at)
VALUES (@workspace_id, @contact_id, 'email', @email::text, false, @now, @now)
ON CONFLICT (workspace_id, type, value) DO NOTHING;
