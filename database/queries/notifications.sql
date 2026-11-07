-- name: ListMemberNotifications :many
-- ListMemberNotifications returns a member's notifications, latest change
-- first, after the cursor when one is given.
SELECT * FROM notifications
WHERE tenant_id = @tenant_id AND notifiable_type = 'workspace_member' AND notifiable_id = @notifiable_id
  AND (NOT @unread_only::boolean OR read_at IS NULL)
  AND (sqlc.narg(trigger)::text IS NULL OR data->>'trigger' = sqlc.narg(trigger))
  AND (sqlc.narg(before_at)::timestamptz IS NULL OR (updated_at, id) < (sqlc.narg(before_at), sqlc.narg(before_id)::uuid))
ORDER BY updated_at DESC, id DESC
LIMIT @row_limit;

-- name: CountMemberNotifications :one
-- CountMemberNotifications counts a member's unread and unseen
-- notifications, each capped so the badge stays cheap.
SELECT
    (SELECT count(*) FROM (SELECT 1 FROM notifications u WHERE u.tenant_id = @tenant_id
        AND u.notifiable_type = 'workspace_member' AND u.notifiable_id = @notifiable_id AND u.read_at IS NULL
        LIMIT 100) x)::bigint AS unread,
    (SELECT count(*) FROM (SELECT 1 FROM notifications s WHERE s.tenant_id = @tenant_id
        AND s.notifiable_type = 'workspace_member' AND s.notifiable_id = @notifiable_id AND s.read_at IS NULL
        AND s.seen_at IS NULL LIMIT 100) y)::bigint AS unseen;

-- name: MarkMemberNotificationsRead :many
-- MarkMemberNotificationsRead marks the member's notifications read: the
-- ones named, or every unread one when none are.
UPDATE notifications SET read_at = @now, seen_at = coalesce(seen_at, @now)
WHERE tenant_id = @tenant_id AND notifiable_type = 'workspace_member' AND notifiable_id = @notifiable_id
  AND read_at IS NULL AND (cardinality(@ids::uuid[]) = 0 OR id = ANY (@ids::uuid[]))
RETURNING id;

-- name: MarkMemberNotificationsUnread :many
-- MarkMemberNotificationsUnread marks the named notifications unread,
-- unless a newer unread one already stands for the same group.
UPDATE notifications AS n SET read_at = NULL
WHERE n.tenant_id = @tenant_id AND n.notifiable_type = 'workspace_member' AND n.notifiable_id = @notifiable_id
  AND n.id = ANY (@ids::uuid[]) AND n.read_at IS NOT NULL
  AND (n.group_key IS NULL OR NOT EXISTS (
      SELECT 1 FROM notifications o WHERE o.tenant_id = n.tenant_id AND o.notifiable_id = n.notifiable_id
        AND o.group_key = n.group_key AND o.read_at IS NULL
  ))
RETURNING n.id;

-- name: MarkMemberNotificationsSeen :exec
-- MarkMemberNotificationsSeen clears the bell's badge for what changed
-- by now.
UPDATE notifications SET seen_at = @now
WHERE tenant_id = @tenant_id AND notifiable_type = 'workspace_member' AND notifiable_id = @notifiable_id
  AND seen_at IS NULL AND updated_at <= @now;

-- name: MarkConversationNotificationsRead :many
-- MarkConversationNotificationsRead marks a member's unread notifications
-- about a conversation read once they read the conversation itself.
UPDATE notifications SET read_at = @now, seen_at = coalesce(seen_at, @now)
WHERE tenant_id = @tenant_id AND notifiable_type = 'workspace_member' AND notifiable_id = @notifiable_id
  AND read_at IS NULL AND data->>'conversation_id' = @conversation_id::text
  AND data->>'trigger' IS DISTINCT FROM 'ai_action_approval_requested'
RETURNING id;

-- name: ResolveApprovalNotifications :exec
-- ResolveApprovalNotifications marks every member's notification about an
-- action call read once someone decided it, so the buttons go away.
UPDATE notifications SET read_at = coalesce(read_at, @now), seen_at = coalesce(seen_at, @now),
    data = data || jsonb_build_object('resolved', coalesce(data->>'resolved', @state::text))
WHERE tenant_id = @tenant_id AND data->>'trigger' = 'ai_action_approval_requested'
  AND data->>'action_call_id' = @action_call_id::text;

-- name: ListNotificationPreferences :many
-- ListNotificationPreferences returns what a member changed from the
-- defaults.
SELECT * FROM notification_preferences
WHERE workspace_id = @workspace_id AND workspace_member_id = @workspace_member_id;

-- name: UpsertNotificationPreference :exec
-- UpsertNotificationPreference records one changed cell.
INSERT INTO notification_preferences (workspace_id, workspace_member_id, trigger, channel, enabled, created_at, updated_at)
VALUES (@workspace_id, @workspace_member_id, @trigger, @channel, @enabled, @now, @now)
ON CONFLICT (workspace_id, workspace_member_id, trigger, channel)
DO UPDATE SET enabled = excluded.enabled, updated_at = excluded.updated_at;

-- name: DeleteNotificationPreference :exec
-- DeleteNotificationPreference forgets a cell set back to its default.
DELETE FROM notification_preferences
WHERE workspace_id = @workspace_id AND workspace_member_id = @workspace_member_id AND trigger = @trigger
  AND channel = @channel;

-- name: DeleteNotificationPreferences :exec
-- DeleteNotificationPreferences puts every cell back to its default.
DELETE FROM notification_preferences WHERE workspace_id = @workspace_id AND workspace_member_id = @workspace_member_id;

-- name: FindNotificationSettings :one
-- FindNotificationSettings returns a member's settings row.
SELECT * FROM notification_settings WHERE workspace_id = @workspace_id AND workspace_member_id = @workspace_member_id;

-- name: UpsertNotificationSettings :one
-- UpsertNotificationSettings writes a member's settings.
INSERT INTO notification_settings (
    workspace_member_id, workspace_id, timezone, quiet_hours_enabled, quiet_hours_start, quiet_hours_end,
    quiet_hours_days, quiet_hours_bypass, paused_until, show_previews, include_ai_handled, created_at, updated_at
) VALUES (
    @workspace_member_id, @workspace_id, @timezone, @quiet_hours_enabled, @quiet_hours_start, @quiet_hours_end,
    @quiet_hours_days, @quiet_hours_bypass, @paused_until, @show_previews, @include_ai_handled, @now, @now
)
ON CONFLICT (workspace_member_id) DO UPDATE SET
    timezone = excluded.timezone, quiet_hours_enabled = excluded.quiet_hours_enabled,
    quiet_hours_start = excluded.quiet_hours_start, quiet_hours_end = excluded.quiet_hours_end,
    quiet_hours_days = excluded.quiet_hours_days, quiet_hours_bypass = excluded.quiet_hours_bypass,
    paused_until = excluded.paused_until, show_previews = excluded.show_previews,
    include_ai_handled = excluded.include_ai_handled, updated_at = excluded.updated_at
RETURNING *;

-- name: ListNotificationRecipients :many
-- ListNotificationRecipients returns active members with what a
-- notification needs to reach them: their user's address, name and
-- language.
SELECT m.id, m.workspace_id, m.user_id, m.role, u.email, u.name, u.locale, u.email_verified_at, u.timezone
FROM workspace_members m JOIN users u ON u.id = m.user_id
WHERE m.workspace_id = @workspace_id AND m.status = 'active'
  AND (cardinality(@ids::uuid[]) = 0 OR m.id = ANY (@ids::uuid[]));

-- name: UpsertPushSubscription :one
-- UpsertPushSubscription stores a browser's push subscription, taking it
-- over when the same endpoint registers again.
INSERT INTO push_subscriptions (
    user_id, session_id, endpoint, endpoint_hash, p256dh, auth_secret_encrypted, vapid_key_id, platform, user_agent,
    device_label, created_at, updated_at
) VALUES (
    @user_id, @session_id, @endpoint, @endpoint_hash, @p256dh, @auth_secret_encrypted, @vapid_key_id, @platform,
    @user_agent, @device_label, @now, @now
)
ON CONFLICT (endpoint_hash) DO UPDATE SET
    user_id = excluded.user_id, session_id = excluded.session_id, p256dh = excluded.p256dh,
    auth_secret_encrypted = excluded.auth_secret_encrypted, vapid_key_id = excluded.vapid_key_id,
    user_agent = excluded.user_agent, device_label = excluded.device_label, consecutive_failures = 0,
    updated_at = excluded.updated_at
RETURNING *;

-- name: ListUserPushSubscriptions :many
-- ListUserPushSubscriptions returns a user's devices, newest first.
SELECT * FROM push_subscriptions WHERE user_id = @user_id ORDER BY created_at DESC;

-- name: DeleteUserPushSubscription :execrows
-- DeleteUserPushSubscription removes one of the user's devices.
DELETE FROM push_subscriptions WHERE user_id = @user_id AND id = @id;

-- name: DeletePushSubscriptionByEndpoint :exec
-- DeletePushSubscriptionByEndpoint removes a subscription the push
-- service says is gone.
DELETE FROM push_subscriptions WHERE endpoint_hash = @endpoint_hash;

-- name: TouchPushSubscription :exec
-- TouchPushSubscription notes a push went through.
UPDATE push_subscriptions SET last_success_at = @now, consecutive_failures = 0
WHERE endpoint_hash = @endpoint_hash;

-- name: NotificationConversationState :one
-- NotificationConversationState is what a notification about a
-- conversation checks before it goes out: where the conversation is,
-- when the member last read it, and whether a customer wrote again or
-- someone answered since the moment it is about.
SELECT c.status, c.spam, c.deleted_at, c.assignee_member_id, c.assignee_team_id, c.inbox_id, c.ai_state,
    coalesce((SELECT r.last_read_at FROM conversation_reads r WHERE r.conversation_id = c.id
        AND r.workspace_member_id = @member_id), 'epoch'::timestamptz)::timestamptz AS last_read_at,
    EXISTS (SELECT 1 FROM messages m WHERE m.workspace_id = c.workspace_id AND m.conversation_id = c.id
        AND m.type = 'message' AND m.author_type = 'contact' AND m.deleted_at IS NULL
        AND m.created_at > @since::timestamptz) AS customer_wrote_since,
    EXISTS (SELECT 1 FROM messages m WHERE m.workspace_id = c.workspace_id AND m.conversation_id = c.id
        AND m.type = 'message' AND m.author_type IN ('member', 'agent') AND m.deleted_at IS NULL
        AND m.created_at > @since::timestamptz) AS answered_since
FROM conversations c
WHERE c.workspace_id = @workspace_id AND c.id = @conversation_id;

-- name: FirstCustomerMessageID :one
-- FirstCustomerMessageID names the customer's first message in the
-- conversation, which made it a new conversation.
SELECT id FROM messages
WHERE workspace_id = @workspace_id AND conversation_id = @conversation_id AND type = 'message' AND author_type = 'contact'
ORDER BY created_at, id
LIMIT 1;
