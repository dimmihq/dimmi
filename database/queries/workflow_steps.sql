-- name: ReplyTimeFacts :one
-- ReplyTimeFacts returns what a reply time is worked out from: the open
-- conversations waiting for the team (or one team), the teammates who
-- are available to answer them, and the median first reply of the last
-- fourteen days, in seconds, zero when there were none.
SELECT
    (SELECT count(*) FROM conversations c
     WHERE c.workspace_id = @workspace_id AND c.status = 'open' AND c.deleted_at IS NULL
       AND c.waiting_since IS NOT NULL
       AND (sqlc.narg(team_id)::uuid IS NULL OR c.assignee_team_id = sqlc.narg(team_id)::uuid))::bigint AS waiting,
    (SELECT count(*) FROM workspace_members m
     WHERE m.workspace_id = @workspace_id AND m.status = 'active' AND m.availability = 'online'
       AND (sqlc.narg(team_id)::uuid IS NULL OR EXISTS (
           SELECT 1 FROM team_members tm
           WHERE tm.workspace_id = m.workspace_id AND tm.workspace_member_id = m.id
             AND tm.team_id = sqlc.narg(team_id)::uuid)))::bigint AS available,
    COALESCE((SELECT percentile_cont(0.5) WITHIN GROUP (ORDER BY extract(epoch FROM c.first_reply_at - c.created_at))
     FROM conversations c
     WHERE c.workspace_id = @workspace_id AND c.first_reply_at IS NOT NULL AND c.deleted_at IS NULL
       AND c.created_at >= @since
       AND (sqlc.narg(team_id)::uuid IS NULL OR c.assignee_team_id = sqlc.narg(team_id)::uuid)), 0)::float8
        AS median_seconds;

-- name: PassConversationClaim :execrows
-- PassConversationClaim hands the conversation a run holds to another
-- run: to a workflow it runs, and back once that one ends.
UPDATE conversations SET workflow_run_id = @to_run_id, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @conversation_id AND workflow_run_id = @from_run_id;
