-- name: SetConversationSLAPolicy :exec
-- SetConversationSLAPolicy gives a plain conversation a service level
-- policy, replacing the one it had.
INSERT INTO conversation_sla_policies (conversation_id, workspace_id, policy_id, created_at, updated_at)
VALUES (@conversation_id, @workspace_id, @policy_id, @now, @now)
ON CONFLICT (conversation_id) DO UPDATE SET policy_id = excluded.policy_id, updated_at = excluded.updated_at;

-- name: ConversationSLAPolicyID :one
-- ConversationSLAPolicyID returns the policy a plain conversation follows.
SELECT policy_id FROM conversation_sla_policies
WHERE workspace_id = @workspace_id AND conversation_id = @conversation_id;
