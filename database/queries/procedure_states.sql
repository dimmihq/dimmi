-- name: FindActiveProcedureState :one
-- FindActiveProcedureState returns the procedure a conversation is
-- following now.
SELECT * FROM conversation_procedure_states
WHERE workspace_id = @workspace_id AND conversation_id = @conversation_id AND status = 'active';

-- name: StartProcedureState :one
-- StartProcedureState records that a conversation began following a
-- procedure.
INSERT INTO conversation_procedure_states (
    workspace_id, conversation_id, procedure_id, procedure_version_id, status, completed_step_ids,
    called_action_ids, step_outputs, branches, started_run_id, last_run_id, started_at, updated_at, ended_at
) VALUES (
    @workspace_id, @conversation_id, @procedure_id, @procedure_version_id, @status, @completed_step_ids,
    @called_action_ids, @step_outputs, @branches, @run_id, @run_id, @now, @now, sqlc.narg(ended_at)
)
RETURNING *;

-- name: UpdateProcedureState :one
-- UpdateProcedureState records how far a conversation got in its
-- procedure, and whether it ended.
UPDATE conversation_procedure_states SET
    status = @status, completed_step_ids = @completed_step_ids, called_action_ids = @called_action_ids,
    step_outputs = @step_outputs, branches = @branches, last_run_id = @run_id, updated_at = @now,
    ended_at = sqlc.narg(ended_at)
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: EndProcedureStates :exec
-- EndProcedureStates ends whatever procedure a conversation follows.
UPDATE conversation_procedure_states SET status = @status, updated_at = @now, ended_at = @now
WHERE workspace_id = @workspace_id AND conversation_id = @conversation_id AND status = 'active';

-- name: MemberRepliedSince :one
-- MemberRepliedSince reports whether a teammate wrote to the customer in
-- the conversation after a moment.
SELECT EXISTS (
    SELECT 1 FROM messages
    WHERE workspace_id = @workspace_id AND conversation_id = @conversation_id AND author_type = 'member'
        AND type = 'message' AND created_at > @since
)::boolean;
