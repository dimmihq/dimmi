-- name: ListWorkflows :many
-- ListWorkflows returns a workspace's workflows in list order, archived
-- ones only when asked.
SELECT * FROM workflows
WHERE workspace_id = @workspace_id
  AND (CASE WHEN @archived::boolean THEN deleted_at IS NOT NULL ELSE deleted_at IS NULL END)
ORDER BY position, created_at;

-- name: FindWorkflow :one
-- FindWorkflow returns one workflow, archived or not.
SELECT * FROM workflows WHERE workspace_id = @workspace_id AND id = @id;

-- name: LockWorkflow :one
-- LockWorkflow returns the workflow and holds it until the transaction
-- ends.
SELECT * FROM workflows WHERE workspace_id = @workspace_id AND id = @id FOR UPDATE;

-- name: WorkflowNameTaken :one
-- WorkflowNameTaken reports whether another live workflow has the name.
SELECT EXISTS (
    SELECT 1 FROM workflows
    WHERE workspace_id = @workspace_id AND lower(name) = lower(@name) AND deleted_at IS NULL AND id <> @id
) AS taken;

-- name: NextWorkflowPosition :one
-- NextWorkflowPosition is where a new workflow goes: the end of the list.
SELECT (coalesce(max(position), 0) + 1024)::integer AS position FROM workflows
WHERE workspace_id = @workspace_id AND deleted_at IS NULL;

-- name: CountWorkflows :one
-- CountWorkflows counts the workspace's workflows that are not archived,
-- and how many of them are live.
SELECT count(*)::integer AS total, (count(*) FILTER (WHERE status = 'live'))::integer AS live
FROM workflows WHERE workspace_id = @workspace_id AND deleted_at IS NULL;

-- name: CreateWorkflow :one
-- CreateWorkflow stores a new draft workflow.
INSERT INTO workflows (
    id, workspace_id, name, description, trigger, position, draft, draft_hash, kind,
    created_by_member_id, updated_by_member_id, created_at, updated_at
) VALUES (
    @id, @workspace_id, @name, @description, @trigger, @position, @draft, @draft_hash, @kind,
    @member_id, @member_id, @now, @now
)
RETURNING *;

-- name: UpdateWorkflowDraft :one
-- UpdateWorkflowDraft writes the working copy and its name, if nobody
-- changed it since the lock version the editor read.
UPDATE workflows SET
    name = @name, description = @description, draft = @draft, draft_hash = @draft_hash, kind = @kind,
    test_on_test_conversations = @test_on_test_conversations, updated_by_member_id = @member_id,
    lock_version = lock_version + 1, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL AND lock_version = @lock_version
RETURNING *;

-- name: SetWorkflowLive :one
-- SetWorkflowLive points the workflow at a published version and makes
-- it live.
UPDATE workflows SET
    live_version_id = @version_id, live_hash = @live_hash, status = 'live', published_at = @now::timestamptz, paused_at = NULL,
    updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: SetWorkflowStatus :one
-- SetWorkflowStatus pauses or resumes a workflow.
UPDATE workflows SET
    status = @status,
    paused_at = CASE WHEN @status = 'paused' THEN @now::timestamptz ELSE NULL END,
    updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: ArchiveWorkflow :one
-- ArchiveWorkflow archives a workflow; it stops matching at once.
UPDATE workflows SET deleted_at = @now::timestamptz, status = CASE WHEN status = 'live' THEN 'paused' ELSE status END,
    updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL
RETURNING *;

-- name: UnarchiveWorkflow :one
-- UnarchiveWorkflow brings an archived workflow back, paused or draft,
-- at the end of the list.
UPDATE workflows SET deleted_at = NULL, position = @position, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NOT NULL
RETURNING *;

-- name: SetWorkflowPosition :exec
-- SetWorkflowPosition moves a workflow in the list.
UPDATE workflows SET position = @position, updated_at = @now WHERE workspace_id = @workspace_id AND id = @id;

-- name: NextWorkflowVersionNumber :one
-- NextWorkflowVersionNumber is the number the workflow's next version
-- takes.
SELECT (coalesce(max(number), 0) + 1)::integer AS number FROM workflow_versions
WHERE workspace_id = @workspace_id AND workflow_id = @workflow_id;

-- name: CreateWorkflowVersion :one
-- CreateWorkflowVersion freezes a definition as a version.
INSERT INTO workflow_versions (
    id, workspace_id, workflow_id, number, spec, spec_hash, kind, compiled, note, published_by_member_id,
    published_via, restored_from_version_id, created_at
) VALUES (
    @id, @workspace_id, @workflow_id, @number, @spec, @spec_hash, @kind, @compiled, @note, @member_id,
    @published_via, @restored_from_version_id, @now
)
RETURNING *;

-- name: FindWorkflowVersion :one
-- FindWorkflowVersion returns one version of a workflow.
SELECT * FROM workflow_versions WHERE workspace_id = @workspace_id AND id = @id;

-- name: ListWorkflowVersions :many
-- ListWorkflowVersions returns a workflow's versions, newest first.
SELECT * FROM workflow_versions
WHERE workspace_id = @workspace_id AND workflow_id = @workflow_id
ORDER BY number DESC
LIMIT @row_limit;

-- name: ListLiveWorkflowVersions :many
-- ListLiveWorkflowVersions returns every live workflow of a trigger with
-- its live version, in list order: the candidates for an event.
SELECT w.id AS workflow_id, w.name, w.position, w.created_at AS workflow_created_at,
    v.id AS version_id, v.number, v.spec, v.kind
FROM workflows w
JOIN workflow_versions v ON v.id = w.live_version_id
WHERE w.workspace_id = @workspace_id AND w.trigger = @trigger AND w.status = 'live' AND w.deleted_at IS NULL
ORDER BY w.position, w.created_at;

-- name: ListTestWorkflowDrafts :many
-- ListTestWorkflowDrafts returns the drafts of a trigger that run on test
-- conversations, in list order.
SELECT * FROM workflows
WHERE workspace_id = @workspace_id AND trigger = @trigger AND test_on_test_conversations AND deleted_at IS NULL
ORDER BY position, created_at;

-- name: WorkflowsCallingWorkflow :many
-- WorkflowsCallingWorkflow returns the live workflows whose live version
-- calls the workflow, which may therefore not be archived.
SELECT w.id, w.name FROM workflows w
JOIN workflow_versions v ON v.id = w.live_version_id
WHERE w.workspace_id = @workspace_id AND w.status = 'live' AND w.deleted_at IS NULL AND w.id <> @workflow_id
  AND v.compiled -> 'references' -> 'workflows' ? @public_id::text;

-- name: MissingWorkflowReferences :many
-- MissingWorkflowReferences returns the referenced ids that name no live
-- team, active member, tag, schedule or saved audience of the workspace.
SELECT r.kind::text AS kind, r.id::uuid AS id FROM (
    SELECT 'team' AS kind, x AS id FROM unnest(@team_ids::uuid[]) AS x
    UNION ALL SELECT 'member', x FROM unnest(@member_ids::uuid[]) AS x
    UNION ALL SELECT 'tag', x FROM unnest(@tag_ids::uuid[]) AS x
    UNION ALL SELECT 'schedule', x FROM unnest(@schedule_ids::uuid[]) AS x
    UNION ALL SELECT 'audience', x FROM unnest(@audience_ids::uuid[]) AS x
) r
WHERE NOT CASE r.kind
    WHEN 'team' THEN EXISTS (
        SELECT 1 FROM teams t WHERE t.workspace_id = @workspace_id AND t.id = r.id AND t.deleted_at IS NULL)
    WHEN 'member' THEN EXISTS (
        SELECT 1 FROM workspace_members m WHERE m.workspace_id = @workspace_id AND m.id = r.id
          AND m.status = 'active')
    WHEN 'tag' THEN EXISTS (SELECT 1 FROM tags g WHERE g.workspace_id = @workspace_id AND g.id = r.id)
    WHEN 'audience' THEN EXISTS (
        SELECT 1 FROM audiences a WHERE a.workspace_id = @workspace_id AND a.id = r.id AND a.deleted_at IS NULL)
    ELSE EXISTS (
        SELECT 1 FROM schedules s WHERE s.workspace_id = @workspace_id AND s.id = r.id AND s.deleted_at IS NULL)
END;

-- name: LiveReusableWorkflows :many
-- LiveReusableWorkflows returns the live reusable workflows among the ids,
-- with their kind.
SELECT w.id, v.kind FROM workflows w
JOIN workflow_versions v ON v.id = w.live_version_id
WHERE w.workspace_id = @workspace_id AND w.id = ANY (@ids::uuid[]) AND w.trigger = 'reusable'
  AND w.status = 'live' AND w.deleted_at IS NULL;
