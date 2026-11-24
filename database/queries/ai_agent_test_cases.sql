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
    simulated_at, scenario, created_by_member_id, created_at, updated_at
) VALUES (
    @workspace_id, @agent_id, @name, sqlc.narg(source_conversation_id), @persona, @messages, @action_mocks,
    @assertions, sqlc.narg(simulated_at), @scenario, sqlc.narg(member_id), @now, @now
)
RETURNING *;

-- name: UpdateAgentTestCase :one
-- UpdateAgentTestCase saves a changed simulation.
UPDATE ai_agent_test_cases SET name = @name, persona = @persona, messages = @messages,
    action_mocks = @action_mocks, assertions = @assertions, simulated_at = sqlc.narg(simulated_at),
    scenario = @scenario, updated_at = @now
WHERE workspace_id = @workspace_id AND agent_id = @agent_id AND id = @id
RETURNING *;

-- name: SetAgentTestCaseDraftResult :exec
-- SetAgentTestCaseDraftResult keeps how a simulation last went on the
-- agent's draft, and its recent history there.
UPDATE ai_agent_test_cases SET draft_result = @result, draft_history = @history
WHERE workspace_id = @workspace_id AND id = @id;

-- name: SetAgentTestCaseLiveResult :exec
-- SetAgentTestCaseLiveResult keeps how a simulation last went on the
-- agent's live version, and its recent history there.
UPDATE ai_agent_test_cases SET live_result = @result, live_history = @history
WHERE workspace_id = @workspace_id AND id = @id;

-- name: InsertAgentTestCaseRun :one
-- InsertAgentTestCaseRun keeps one run of a simulation.
INSERT INTO ai_agent_test_case_runs (
    workspace_id, test_case_id, version, fingerprint, passed, errored, result, ran_at
) VALUES (
    @workspace_id, @test_case_id, @version, @fingerprint, @passed, @errored, @result, @ran_at
)
RETURNING *;

-- name: ListAgentTestCaseRuns :many
-- ListAgentTestCaseRuns returns a simulation's runs on one copy of the
-- agent, newest first.
SELECT * FROM ai_agent_test_case_runs
WHERE workspace_id = @workspace_id AND test_case_id = @test_case_id AND version = @version
ORDER BY ran_at DESC, id DESC
LIMIT @row_limit;

-- name: PruneAgentTestCaseRuns :exec
-- PruneAgentTestCaseRuns drops a simulation's runs on one copy past the
-- newest it keeps.
DELETE FROM ai_agent_test_case_runs r
WHERE r.workspace_id = @workspace_id AND r.test_case_id = @test_case_id AND r.version = @version
  AND r.id NOT IN (
      SELECT k.id FROM ai_agent_test_case_runs k
      WHERE k.workspace_id = @workspace_id AND k.test_case_id = @test_case_id AND k.version = @version
      ORDER BY k.ran_at DESC, k.id DESC
      LIMIT @keep
  );

-- name: DeleteAgentTestCase :exec
-- DeleteAgentTestCase removes a simulation.
DELETE FROM ai_agent_test_cases WHERE workspace_id = @workspace_id AND agent_id = @agent_id AND id = @id;
