-- name: ListProcedureTestCases :many
-- ListProcedureTestCases returns a procedure's saved tests, oldest first.
SELECT * FROM procedure_test_cases
WHERE workspace_id = @workspace_id AND procedure_id = @procedure_id
ORDER BY created_at, id;

-- name: FindProcedureTestCase :one
-- FindProcedureTestCase returns one saved test of a procedure.
SELECT * FROM procedure_test_cases
WHERE workspace_id = @workspace_id AND procedure_id = @procedure_id AND id = @id;

-- name: CreateProcedureTestCase :one
-- CreateProcedureTestCase saves a test of a procedure.
INSERT INTO procedure_test_cases (
    workspace_id, procedure_id, name, source_conversation_id, persona, messages, action_mocks, assertions,
    created_by_member_id, created_at, updated_at
) VALUES (
    @workspace_id, @procedure_id, @name, sqlc.narg(source_conversation_id), @persona, @messages, @action_mocks,
    @assertions, sqlc.narg(member_id), @now, @now
)
RETURNING *;

-- name: UpdateProcedureTestCase :one
-- UpdateProcedureTestCase saves a changed test.
UPDATE procedure_test_cases SET name = @name, persona = @persona, messages = @messages,
    action_mocks = @action_mocks, assertions = @assertions, updated_at = @now
WHERE workspace_id = @workspace_id AND procedure_id = @procedure_id AND id = @id
RETURNING *;

-- name: SetProcedureTestCaseResult :exec
-- SetProcedureTestCaseResult keeps how a test's last run went.
UPDATE procedure_test_cases SET last_result = @last_result
WHERE workspace_id = @workspace_id AND id = @id;

-- name: DeleteProcedureTestCase :exec
-- DeleteProcedureTestCase removes a saved test.
DELETE FROM procedure_test_cases WHERE workspace_id = @workspace_id AND procedure_id = @procedure_id AND id = @id;
