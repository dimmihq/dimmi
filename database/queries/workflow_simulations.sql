-- name: ListWorkflowSimulations :many
-- ListWorkflowSimulations returns a workflow's simulations, oldest first.
SELECT * FROM workflow_simulations
WHERE workspace_id = @workspace_id AND workflow_id = @workflow_id
ORDER BY created_at, id;

-- name: FindWorkflowSimulation :one
-- FindWorkflowSimulation returns one simulation of a workflow.
SELECT * FROM workflow_simulations
WHERE workspace_id = @workspace_id AND workflow_id = @workflow_id AND id = @id;

-- name: CreateWorkflowSimulation :one
-- CreateWorkflowSimulation saves a simulation of a workflow.
INSERT INTO workflow_simulations (
    workspace_id, workflow_id, name, source_conversation_id, persona, messages, action_mocks, assertions,
    simulated_at, scenario, created_by_member_id, created_at, updated_at
) VALUES (
    @workspace_id, @workflow_id, @name, sqlc.narg(source_conversation_id), @persona, @messages, @action_mocks,
    @assertions, sqlc.narg(simulated_at), @scenario, sqlc.narg(member_id), @now, @now
)
RETURNING *;

-- name: UpdateWorkflowSimulation :one
-- UpdateWorkflowSimulation saves a changed simulation.
UPDATE workflow_simulations SET name = @name, persona = @persona, messages = @messages,
    action_mocks = @action_mocks, assertions = @assertions, simulated_at = sqlc.narg(simulated_at),
    scenario = @scenario, updated_at = @now
WHERE workspace_id = @workspace_id AND workflow_id = @workflow_id AND id = @id
RETURNING *;

-- name: SetWorkflowSimulationDraftResult :exec
-- SetWorkflowSimulationDraftResult keeps how a simulation last went on
-- the workflow's draft, and its recent history there.
UPDATE workflow_simulations SET draft_result = @result, draft_history = @history
WHERE workspace_id = @workspace_id AND id = @id;

-- name: SetWorkflowSimulationLiveResult :exec
-- SetWorkflowSimulationLiveResult keeps how a simulation last went on
-- the workflow's live version, and its recent history there.
UPDATE workflow_simulations SET live_result = @result, live_history = @history
WHERE workspace_id = @workspace_id AND id = @id;

-- name: InsertWorkflowSimulationRun :one
-- InsertWorkflowSimulationRun keeps one run of a simulation.
INSERT INTO workflow_simulation_runs (
    workspace_id, simulation_id, version, fingerprint, passed, errored, result, ran_at
) VALUES (
    @workspace_id, @simulation_id, @version, @fingerprint, @passed, @errored, @result, @ran_at
)
RETURNING *;

-- name: ListWorkflowSimulationRuns :many
-- ListWorkflowSimulationRuns returns a simulation's runs on one copy of
-- the workflow, newest first.
SELECT * FROM workflow_simulation_runs
WHERE workspace_id = @workspace_id AND simulation_id = @simulation_id AND version = @version
ORDER BY ran_at DESC, id DESC
LIMIT @row_limit;

-- name: PruneWorkflowSimulationRuns :exec
-- PruneWorkflowSimulationRuns drops a simulation's runs on one copy past
-- the newest it keeps.
DELETE FROM workflow_simulation_runs r
WHERE r.workspace_id = @workspace_id AND r.simulation_id = @simulation_id AND r.version = @version
  AND r.id NOT IN (
      SELECT k.id FROM workflow_simulation_runs k
      WHERE k.workspace_id = @workspace_id AND k.simulation_id = @simulation_id AND k.version = @version
      ORDER BY k.ran_at DESC, k.id DESC
      LIMIT @keep
  );

-- name: DeleteWorkflowSimulation :exec
-- DeleteWorkflowSimulation removes a simulation.
DELETE FROM workflow_simulations WHERE workspace_id = @workspace_id AND workflow_id = @workflow_id AND id = @id;
