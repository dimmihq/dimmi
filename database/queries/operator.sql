-- name: FindOperatorSettings :one
-- FindOperatorSettings returns the workspace's Operator settings.
SELECT * FROM operator_settings WHERE workspace_id = @workspace_id;

-- name: SaveOperatorSettings :one
-- SaveOperatorSettings writes the workspace's Operator settings whole.
INSERT INTO operator_settings (
    workspace_id, enabled, brief_on_open, triage_on_arrival, instructions, policies, model, created_at, updated_at
) VALUES (
    @workspace_id, @enabled, @brief_on_open, @triage_on_arrival, @instructions, @policies, sqlc.narg('model'),
    @now, @now
)
ON CONFLICT (workspace_id) DO UPDATE SET
    enabled = EXCLUDED.enabled, brief_on_open = EXCLUDED.brief_on_open,
    triage_on_arrival = EXCLUDED.triage_on_arrival, instructions = EXCLUDED.instructions,
    policies = EXCLUDED.policies, model = EXCLUDED.model, updated_at = EXCLUDED.updated_at
RETURNING *;

-- name: CreateOperatorThread :one
-- CreateOperatorThread starts a new thread for a member, about one
-- conversation or the whole workspace.
INSERT INTO operator_threads (workspace_id, member_id, conversation_id, title, created_at, updated_at)
VALUES (@workspace_id, @member_id, sqlc.narg('conversation_id'), @title, @now, @now)
RETURNING *;

-- name: FindOperatorThread :one
-- FindOperatorThread returns one of the member's threads.
SELECT * FROM operator_threads WHERE workspace_id = @workspace_id AND member_id = @member_id AND id = @id;

-- name: FindConversationOperatorThread :one
-- FindConversationOperatorThread returns the member's latest thread
-- about the conversation.
SELECT * FROM operator_threads
WHERE workspace_id = @workspace_id AND member_id = @member_id AND conversation_id = @conversation_id
ORDER BY updated_at DESC, id DESC
LIMIT 1;

-- name: ListOperatorThreads :many
-- ListOperatorThreads returns the member's threads with messages, latest
-- first: only those about the whole workspace when asked, or only those
-- about one conversation when one is named.
SELECT * FROM operator_threads
WHERE workspace_id = @workspace_id AND member_id = @member_id
  AND (NOT @workspace_only::boolean OR conversation_id IS NULL)
  AND (sqlc.narg('conversation_id')::uuid IS NULL OR conversation_id = sqlc.narg('conversation_id')::uuid)
  AND last_message_at IS NOT NULL
ORDER BY last_message_at DESC, id DESC
LIMIT @row_limit;

-- name: TouchOperatorThread :exec
-- TouchOperatorThread records a new message on the thread and names it
-- when it had no title.
UPDATE operator_threads
SET last_message_at = @now::timestamptz, updated_at = @now::timestamptz,
    title = CASE WHEN title = '' THEN @title ELSE title END
WHERE workspace_id = @workspace_id AND id = @id;

-- name: DeleteOperatorThread :execrows
-- DeleteOperatorThread removes one of the member's threads with its
-- messages.
DELETE FROM operator_threads WHERE workspace_id = @workspace_id AND member_id = @member_id AND id = @id;

-- name: CreateOperatorMessage :one
-- CreateOperatorMessage stores one turn of a thread.
INSERT INTO operator_messages (workspace_id, thread_id, role, body, steps, status, run_id, created_at)
VALUES (@workspace_id, @thread_id, @role, @body, @steps, @status, sqlc.narg('run_id'), @now)
RETURNING *;

-- name: ListOperatorMessages :many
-- ListOperatorMessages returns a thread's latest messages, newest first.
SELECT * FROM operator_messages
WHERE workspace_id = @workspace_id AND thread_id = @thread_id
ORDER BY created_at DESC, id DESC
LIMIT @row_limit;

-- name: CreateOperatorChangeset :one
-- CreateOperatorChangeset stores the changes Operator wants to make.
INSERT INTO operator_changesets (
    workspace_id, thread_id, message_id, member_id, conversation_id, title, reason, operations, status, auto,
    routine_id, routine_run_id, dedup_key, preview, expires_at, created_at, updated_at
) VALUES (
    @workspace_id, sqlc.narg('thread_id'), sqlc.narg('message_id'), sqlc.narg('member_id'),
    sqlc.narg('conversation_id'), @title, @reason, @operations, 'pending', @auto, sqlc.narg('routine_id'),
    sqlc.narg('routine_run_id'), sqlc.narg('dedup_key'), @preview, sqlc.narg('expires_at'), @now, @now
)
RETURNING *;

-- name: FindOperatorChangeset :one
-- FindOperatorChangeset returns one changeset.
SELECT * FROM operator_changesets WHERE workspace_id = @workspace_id AND id = @id;

-- name: LockOperatorChangeset :one
-- LockOperatorChangeset returns one changeset, held until the
-- transaction ends so two teammates never apply it twice.
SELECT * FROM operator_changesets WHERE workspace_id = @workspace_id AND id = @id FOR UPDATE;

-- name: AttachOperatorChangesets :exec
-- AttachOperatorChangesets ties the changesets a run proposed to the
-- message that answered.
UPDATE operator_changesets SET message_id = @message_id
WHERE workspace_id = @workspace_id AND id = ANY (@ids::uuid[]);

-- name: ListOperatorChangesets :many
-- ListOperatorChangesets returns the changesets a member may review,
-- newest first: their own, those Operator proposed on its own about
-- conversations they can see, and those a routine left when the member
-- reads what the routine reads; of one conversation, thread, routine or
-- status, or since a moment, when asked. A routine reads every inbox
-- when it names none, so only members who see every conversation read
-- its suggestions; one that names inboxes is read by members who see all
-- of them.
SELECT s.* FROM operator_changesets s
LEFT JOIN conversations c ON c.id = s.conversation_id
LEFT JOIN operator_routines r ON r.id = s.routine_id
WHERE s.workspace_id = @workspace_id
  AND (s.member_id = @member_id
    OR (s.member_id IS NULL AND s.routine_id IS NULL AND s.conversation_id IS NOT NULL)
    OR (s.member_id IS NULL AND s.routine_id IS NOT NULL AND @reads_routines::boolean
      AND (@sees_everything::boolean OR (@sees_all_modes::boolean AND cardinality(r.inbox_ids) > 0
        AND NOT (r.inbox_ids && @access_hidden_inboxes::uuid[])))))
  AND (sqlc.narg('conversation_id')::uuid IS NULL OR s.conversation_id = sqlc.narg('conversation_id')::uuid)
  AND (sqlc.narg('thread_id')::uuid IS NULL OR s.thread_id = sqlc.narg('thread_id')::uuid)
  AND (sqlc.narg('routine_id')::uuid IS NULL OR s.routine_id = sqlc.narg('routine_id')::uuid)
  AND (sqlc.narg('status')::text IS NULL OR s.status = sqlc.narg('status')::text)
  AND (sqlc.narg('since')::timestamptz IS NULL OR s.created_at >= sqlc.narg('since')::timestamptz)
  AND (NOT @without_previews::boolean OR NOT s.preview)
  AND (s.conversation_id IS NULL OR (c.deleted_at IS NULL AND dimmi_conversation_visible(
      c.assignee_member_id, c.assignee_team_id, c.inbox_id, @access_mode::text,
      sqlc.narg(access_viewer)::uuid, @access_teams::uuid[], @access_unassigned::boolean,
      @access_hidden_inboxes::uuid[]
  )))
ORDER BY s.created_at DESC, s.id DESC
LIMIT @row_limit;

-- name: SettleOperatorChangeset :one
-- SettleOperatorChangeset records what became of a changeset.
UPDATE operator_changesets
SET status = @status, operations = @operations, error = sqlc.narg('error'),
    decided_by_member_id = sqlc.narg('decided_by_member_id'),
    applied_at = COALESCE(sqlc.narg('applied_at'), applied_at),
    reverted_at = COALESCE(sqlc.narg('reverted_at'), reverted_at), updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: CreateOperatorMemory :one
-- CreateOperatorMemory keeps something a member asked Operator to
-- remember.
INSERT INTO operator_memories (workspace_id, member_id, body, created_at)
VALUES (@workspace_id, @member_id, @body, @now)
RETURNING *;

-- name: ListOperatorMemories :many
-- ListOperatorMemories returns what the member asked Operator to
-- remember, newest first.
SELECT * FROM operator_memories
WHERE workspace_id = @workspace_id AND member_id = @member_id
ORDER BY created_at DESC, id DESC
LIMIT @row_limit;

-- name: DeleteOperatorMemory :execrows
-- DeleteOperatorMemory forgets one memory of the member's.
DELETE FROM operator_memories WHERE workspace_id = @workspace_id AND member_id = @member_id AND id = @id;

-- name: SetOperatorToolLayer :exec
-- SetOperatorToolLayer turns Operator's tool layer on or off.
INSERT INTO operator_settings (workspace_id, tool_layer, created_at, updated_at)
VALUES (@workspace_id, @tool_layer, @now, @now)
ON CONFLICT (workspace_id) DO UPDATE SET tool_layer = EXCLUDED.tool_layer, updated_at = EXCLUDED.updated_at;

-- name: SetOperatorCompactResults :exec
-- SetOperatorCompactResults turns Operator's compact results on or off.
INSERT INTO operator_settings (workspace_id, compact_results, created_at, updated_at)
VALUES (@workspace_id, @compact_results, @now, @now)
ON CONFLICT (workspace_id) DO UPDATE SET compact_results = EXCLUDED.compact_results, updated_at = EXCLUDED.updated_at;

-- name: ReviseOperatorChangeset :one
-- ReviseOperatorChangeset replaces what a changeset that still waits
-- would change, and why, when Operator amends it for a teammate.
UPDATE operator_changesets
SET operations = @operations, reason = @reason, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND status = 'pending'
RETURNING *;
