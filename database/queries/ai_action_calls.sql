-- name: CreateActionCall :one
-- CreateActionCall records a call the moment it is requested, in the
-- state its permissions put it in.
INSERT INTO ai_action_calls (
    workspace_id, action_id, action_version_id, connection_id, mode, side_effect, confirmation, approval, state,
    run_id, conversation_id, contact_id, tool_call_id, title, summary, input, bindings, requested_at, sent_at,
    expires_at, attempts, created_at, updated_at
) VALUES (
    @workspace_id, @action_id, @action_version_id, @connection_id, @mode, @side_effect, @confirmation, @approval,
    @state, @run_id, @conversation_id, @contact_id, @tool_call_id, @title, @summary, @input, @bindings, @now::timestamptz,
    @sent_at, @expires_at, @attempts, @now::timestamptz, @now::timestamptz
)
RETURNING *;

-- name: FindActionCall :one
-- FindActionCall returns one call of the workspace.
SELECT * FROM ai_action_calls WHERE workspace_id = @workspace_id AND id = @id;

-- name: FindConversationActionCall :one
-- FindConversationActionCall returns a call only when it belongs to the
-- conversation, so a visitor can never reach another conversation's.
SELECT * FROM ai_action_calls
WHERE workspace_id = @workspace_id AND conversation_id = @conversation_id AND id = @id AND mode = 'live';

-- name: ListActionCalls :many
-- ListActionCalls returns the workspace's calls, newest first, filtered
-- by conversation, action and state, starting after a cursor.
SELECT * FROM ai_action_calls
WHERE workspace_id = @workspace_id
    AND (sqlc.narg(conversation_id)::uuid IS NULL OR conversation_id = sqlc.narg(conversation_id)::uuid)
    AND (sqlc.narg(action_id)::uuid IS NULL OR action_id = sqlc.narg(action_id)::uuid)
    AND (sqlc.narg(state)::text IS NULL OR state = sqlc.narg(state)::text)
    AND (sqlc.narg(before_id)::uuid IS NULL OR id < sqlc.narg(before_id)::uuid)
ORDER BY id DESC
LIMIT @row_limit;

-- name: ListConversationActionCalls :many
-- ListConversationActionCalls returns a conversation's newest live calls,
-- the ones the agent is told about.
SELECT * FROM ai_action_calls
WHERE workspace_id = @workspace_id AND conversation_id = @conversation_id AND mode = 'live'
ORDER BY id DESC
LIMIT @row_limit;

-- name: ConfirmActionCall :one
-- ConfirmActionCall records the customer's yes to a call still waiting
-- for it and not yet expired, moving it to approval or to execution.
UPDATE ai_action_calls SET
    state = @next_state, confirmed_at = @now::timestamptz, expires_at = @expires_at, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id AND state = 'awaiting_confirmation' AND expires_at > @now::timestamptz
RETURNING *;

-- name: CancelActionCall :one
-- CancelActionCall gives up a call nobody has agreed to run yet.
UPDATE ai_action_calls SET
    state = 'canceled', cancel_reason = @cancel_reason, completed_at = @now::timestamptz, expires_at = NULL, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id AND state IN ('awaiting_confirmation', 'pending_approval')
RETURNING *;

-- name: SupersedeConversationConfirmations :many
-- SupersedeConversationConfirmations cancels the confirmations still
-- waiting in a conversation when the agent proposes a new write.
UPDATE ai_action_calls SET
    state = 'canceled', cancel_reason = 'superseded', completed_at = @now::timestamptz, expires_at = NULL, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND conversation_id = @conversation_id AND state = 'awaiting_confirmation'
RETURNING *;

-- name: DecideActionCall :one
-- DecideActionCall records a teammate's approval or decline of a call
-- still waiting for one and not yet expired.
UPDATE ai_action_calls SET
    state = @next_state, decided_by_member_id = @member_id, decided_at = @now::timestamptz, decision_reason = @decision_reason,
    customer_message = @customer_message, expires_at = NULL,
    completed_at = CASE WHEN @next_state::text = 'declined' THEN @now::timestamptz ELSE completed_at END,
    updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id AND state = 'pending_approval' AND expires_at > @now::timestamptz
RETURNING *;

-- name: ExpireActionCall :one
-- ExpireActionCall ends a call whose confirmation or approval did not
-- come in time.
UPDATE ai_action_calls SET state = 'expired', completed_at = @now::timestamptz, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id AND state IN ('awaiting_confirmation', 'pending_approval')
    AND expires_at <= @now::timestamptz
RETURNING *;

-- name: ClaimActionCall :one
-- ClaimActionCall takes an approved call for execution and writes the
-- marker that bytes may leave, before they do. Only one caller ever
-- wins it.
UPDATE ai_action_calls SET state = 'running', sent_at = @now::timestamptz, attempts = attempts + 1, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id AND state = 'approved' AND sent_at IS NULL
RETURNING *;

-- name: FinishActionCall :one
-- FinishActionCall records what a running call came back with.
UPDATE ai_action_calls SET
    state = @state, error_code = @error_code, http_status = @http_status, request = @request,
    response = @response, result = @result, latency_ms = @latency_ms, completed_at = @now::timestamptz, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id AND state = 'running'
RETURNING *;

-- name: AbandonActionCall :one
-- AbandonActionCall fails a call whose execution died after its request
-- may have left: it may have run, so it is never sent again.
UPDATE ai_action_calls SET state = 'failed', error_code = 'outcome_unknown', completed_at = @now::timestamptz, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id AND state = 'running' AND sent_at IS NOT NULL
RETURNING *;

-- name: SetActionCallsMessage :exec
-- SetActionCallsMessage links calls to the agent message that told the
-- customer about them.
UPDATE ai_action_calls SET message_id = @message_id, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = ANY (@ids::uuid[]);

-- name: MarkActionCallReported :exec
-- MarkActionCallReported notes that the customer was told how the call
-- ended, or that nobody needs to be.
UPDATE ai_action_calls SET reported_at = @now::timestamptz, report_run_id = @report_run_id, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id AND reported_at IS NULL;

-- name: SetMessageConfirmationStatus :exec
-- SetMessageConfirmationStatus keeps the confirmation card on an agent
-- message in step with its call.
UPDATE messages SET metadata = jsonb_set(metadata, '{ai,confirmation,status}', to_jsonb(@status::text)),
    updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id AND metadata -> 'ai' ? 'confirmation';

-- name: SetRunTriggerActionCall :exec
-- SetRunTriggerActionCall names the call a follow-up run reports on.
UPDATE ai_runs SET trigger_action_call_id = @action_call_id, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id;

-- name: CountRunActionCall :exec
-- CountRunActionCall adds one action call to the run's tally.
UPDATE ai_runs SET action_calls_count = action_calls_count + 1, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id;

-- name: SetMessageActionCall :exec
-- SetMessageActionCall merges a call's news into the action call an
-- agent message reports, when that message reports this call.
UPDATE messages SET
    metadata = jsonb_set(metadata, '{ai,action_call}', (metadata -> 'ai' -> 'action_call') || @patch::jsonb),
    updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id AND metadata -> 'ai' -> 'action_call' ->> 'id' = @call_id::text;

-- name: CountPendingApprovals :many
-- CountPendingApprovals counts, for each conversation of a page, the
-- live calls waiting for a teammate's approval, so a list row can show
-- them without a query per row.
SELECT conversation_id::uuid AS conversation_id, count(*)::int AS pending
FROM ai_action_calls
WHERE workspace_id = @workspace_id AND state = 'pending_approval' AND mode = 'live'
    AND conversation_id = ANY (@conversation_ids::uuid[])
GROUP BY conversation_id;

-- name: ScrubActionCallsForContact :execrows
-- ScrubActionCallsForContact forgets what an erased customer's action
-- calls held: what was sent, what came back and what people wrote. The
-- rows stay as a record that a call happened, pointing at no one.
UPDATE ai_action_calls SET
    summary = '', input = '{}', bindings = '{}', request = NULL, response = NULL, result = NULL,
    decision_reason = NULL, customer_message = NULL, cancel_reason = NULL, updated_at = @now
WHERE workspace_id = @workspace_id AND contact_id = @contact_id;
