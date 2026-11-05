-- name: EnsureAssignmentCursor :exec
-- EnsureAssignmentCursor creates the pool's cursor when it has none.
INSERT INTO team_assignment_cursors (pool_kind, pool_id, workspace_id, updated_at)
VALUES (@pool_kind, @pool_id, @workspace_id, @now)
ON CONFLICT (pool_kind, pool_id) DO NOTHING;

-- name: LockAssignmentCursor :one
-- LockAssignmentCursor holds the pool's cursor until the transaction
-- ends, so picks in one pool happen one after another.
SELECT * FROM team_assignment_cursors
WHERE workspace_id = @workspace_id AND pool_kind = @pool_kind AND pool_id = @pool_id
FOR UPDATE;

-- name: AdvanceAssignmentCursor :exec
-- AdvanceAssignmentCursor records who the pool's last pick was.
UPDATE team_assignment_cursors
SET last_member_id = @member_id, last_assigned_at = @now, updated_at = @now
WHERE workspace_id = @workspace_id AND pool_kind = @pool_kind AND pool_id = @pool_id;

-- name: DeleteAssignmentCursor :exec
-- DeleteAssignmentCursor forgets the cursor of a pool that is gone.
DELETE FROM team_assignment_cursors
WHERE workspace_id = @workspace_id AND pool_kind = @pool_kind AND pool_id = @pool_id;

-- name: ListPoolCandidates :many
-- ListPoolCandidates returns the active members of the pool, the team's
-- or the whole workspace's, with how many open conversations each holds
-- overall and on the channel type.
SELECT m.id, m.availability, m.conversation_capacity, m.capacity_by_channel, m.last_assigned_at,
    (SELECT count(*) FROM conversations c
     WHERE c.workspace_id = m.workspace_id AND c.assignee_member_id = m.id
       AND c.status = 'open' AND NOT c.spam AND c.deleted_at IS NULL)::integer AS open_count,
    (SELECT count(*) FROM conversations c
     JOIN channel_connections cc ON cc.workspace_id = c.workspace_id AND cc.id = c.channel_connection_id
     WHERE c.workspace_id = m.workspace_id AND c.assignee_member_id = m.id
       AND c.status = 'open' AND NOT c.spam AND c.deleted_at IS NULL
       AND cc.type = @channel_type::text)::integer AS channel_open_count
FROM workspace_members m
WHERE m.workspace_id = @workspace_id AND m.status = 'active'
  AND (sqlc.narg(team_id)::uuid IS NULL OR EXISTS (
      SELECT 1 FROM team_members tm
      WHERE tm.workspace_id = m.workspace_id AND tm.team_id = sqlc.narg(team_id)::uuid
        AND tm.workspace_member_id = m.id
  ))
ORDER BY m.id;

-- name: SetMemberLastAssigned :exec
-- SetMemberLastAssigned records when the member was last given a
-- conversation automatically.
UPDATE workspace_members SET last_assigned_at = @now
WHERE workspace_id = @workspace_id AND id = @id;

-- name: SetConversationAssignmentWait :one
-- SetConversationAssignmentWait says why the conversation waits for an
-- assignee, keeping since when it waits.
UPDATE conversations
SET assignment_wait_reason = @reason, assignment_waiting_since = coalesce(assignment_waiting_since, @now)
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: ClearConversationAssignmentWait :exec
-- ClearConversationAssignmentWait ends the conversation's wait.
UPDATE conversations SET assignment_wait_reason = NULL, assignment_waiting_since = NULL
WHERE workspace_id = @workspace_id AND id = @id AND assignment_wait_reason IS NOT NULL;

-- name: FindNextWaitingConversation :one
-- FindNextWaitingConversation takes the next conversation of the inbox
-- waiting for an assignee, in the inbox's queue order, skipping rows
-- another drain holds and those this drain already tried.
SELECT * FROM conversations
WHERE workspace_id = @workspace_id AND inbox_id = @inbox_id AND assignment_wait_reason IS NOT NULL
  AND NOT (id = ANY (@tried::uuid[]))
ORDER BY CASE WHEN @by_priority::boolean THEN priority END DESC NULLS LAST, assignment_waiting_since, id
LIMIT 1
FOR UPDATE SKIP LOCKED;

-- name: ListWaitingInboxes :many
-- ListWaitingInboxes returns the inboxes of the workspace that have
-- conversations waiting for an assignee.
SELECT DISTINCT inbox_id FROM conversations
WHERE workspace_id = @workspace_id AND assignment_wait_reason IS NOT NULL
ORDER BY inbox_id;

-- name: CountWaitingConversations :many
-- CountWaitingConversations counts the inbox's conversations waiting
-- for an assignee by reason, with the oldest wait of each.
SELECT assignment_wait_reason::text AS reason, count(*)::integer AS waiting,
    min(assignment_waiting_since)::timestamptz AS oldest
FROM conversations
WHERE workspace_id = @workspace_id AND inbox_id = @inbox_id AND assignment_wait_reason IS NOT NULL
GROUP BY assignment_wait_reason
ORDER BY assignment_wait_reason;

-- name: ListAssignmentWaitingWorkspaces :many
-- ListAssignmentWaitingWorkspaces returns the workspaces with any
-- conversation waiting for an assignee.
SELECT workspace_id::uuid FROM assignment_waiting_workspaces(@max_rows::integer);
