-- name: ListAgentTestCases :many
-- ListAgentTestCases returns an agent's simulations, oldest first.
SELECT * FROM ai_agent_test_cases
WHERE workspace_id = @workspace_id AND agent_id = @agent_id
ORDER BY created_at, id;

-- name: FindAgentTestCase :one
-- FindAgentTestCase returns one simulation of an agent.
SELECT * FROM ai_agent_test_cases
WHERE workspace_id = @workspace_id AND agent_id = @agent_id AND id = @id;

-- name: CreateAgentTestCase :one
-- CreateAgentTestCase saves a simulation of an agent.
INSERT INTO ai_agent_test_cases (
    workspace_id, agent_id, name, source_conversation_id, persona, messages, action_mocks, assertions,
    simulated_at, created_by_member_id, created_at, updated_at
) VALUES (
    @workspace_id, @agent_id, @name, sqlc.narg(source_conversation_id), @persona, @messages, @action_mocks,
    @assertions, sqlc.narg(simulated_at), sqlc.narg(member_id), @now, @now
)
RETURNING *;

-- name: UpdateAgentTestCase :one
-- UpdateAgentTestCase saves a changed simulation.
UPDATE ai_agent_test_cases SET name = @name, persona = @persona, messages = @messages,
    action_mocks = @action_mocks, assertions = @assertions, simulated_at = sqlc.narg(simulated_at),
    updated_at = @now
WHERE workspace_id = @workspace_id AND agent_id = @agent_id AND id = @id
RETURNING *;

-- name: SetAgentTestCaseDraftResult :exec
-- SetAgentTestCaseDraftResult keeps how a simulation last went on the
-- agent's draft.
UPDATE ai_agent_test_cases SET draft_result = @result
WHERE workspace_id = @workspace_id AND id = @id;

-- name: SetAgentTestCaseLiveResult :exec
-- SetAgentTestCaseLiveResult keeps how a simulation last went on the
-- agent's live version.
UPDATE ai_agent_test_cases SET live_result = @result
WHERE workspace_id = @workspace_id AND id = @id;

-- name: DeleteAgentTestCase :exec
-- DeleteAgentTestCase removes a simulation.
DELETE FROM ai_agent_test_cases WHERE workspace_id = @workspace_id AND agent_id = @agent_id AND id = @id;
