-- name: ReportConversationInputs :many
-- ReportConversationInputs reads what a report needs of each named
-- conversation itself: where it came in, who holds it, its state and
-- whether it was imported. The facts job asks, no member is.
SELECT c.id, c.workspace_id, c.number, c.inbox_id, ch.type AS channel, c.assignee_member_id, c.assignee_team_id,
    c.priority, c.status, c.spam, c.is_test, c.deleted_at, c.merged_into_id, c.created_at, c.closed_at,
    c.ai_cost_micros, c.ai_handed_off_at, c.ai_handoff_reason,
    COALESCE((c.source ->> 'imported')::boolean, false)::boolean AS imported
FROM conversations c
JOIN channel_connections ch ON ch.id = c.channel_connection_id
WHERE c.workspace_id = @workspace_id AND c.id = ANY (@ids::uuid[]);

-- name: ReportMessageInputs :many
-- ReportMessageInputs reads the public messages of the conversations,
-- oldest first: who wrote each, when it went out and whether it was a
-- real answer. Deleted, undone and not yet sent messages never count.
SELECT m.conversation_id, m.author_type, m.author_id, m.substantive,
    (CASE WHEN m.scheduled_send_at IS NOT NULL THEN COALESCE(m.sent_at, m.scheduled_send_at) ELSE m.created_at END
    )::timestamptz AS at
FROM messages m
WHERE m.workspace_id = @workspace_id AND m.conversation_id = ANY (@ids::uuid[])
  AND m.type = 'message' AND m.deleted_at IS NULL AND m.undone_at IS NULL
  AND (m.scheduled_send_at IS NULL OR m.sent_at IS NOT NULL)
ORDER BY m.conversation_id, at, m.id;

-- name: ReportEventInputs :many
-- ReportEventInputs reads the logged changes of the conversations, in
-- the order they happened.
SELECT conversation_id, kind, occurred_at, member_id, team_id, reason
FROM conversation_events
WHERE workspace_id = @workspace_id AND conversation_id = ANY (@ids::uuid[])
ORDER BY conversation_id, occurred_at, id;

-- name: ReportRatingInputs :many
-- ReportRatingInputs reads the latest answered rating of each of the
-- conversations.
SELECT DISTINCT ON (conversation_id) conversation_id, score::smallint AS score, rated_at::timestamptz AS rated_at,
    member_id
FROM conversation_ratings
WHERE workspace_id = @workspace_id AND conversation_id = ANY (@ids::uuid[]) AND score IS NOT NULL
  AND rated_at IS NOT NULL
ORDER BY conversation_id, rated_at DESC;

-- name: ReportHelpedInputs :many
-- ReportHelpedInputs returns which of the conversations had a customer
-- say an AI answer helped.
SELECT DISTINCT conversation_id::uuid AS conversation_id
FROM ai_runs
WHERE workspace_id = @workspace_id AND conversation_id = ANY (@ids::uuid[]) AND feedback = 'helped';

-- name: ReportInboxSchedules :many
-- ReportInboxSchedules returns each inbox's office hours: its own
-- schedule, else the workspace's default, none when there is neither.
SELECT i.id AS inbox_id, s.timezone, s.weekly, s.holidays
FROM inboxes i
LEFT JOIN schedules s ON s.workspace_id = i.workspace_id AND s.deleted_at IS NULL AND s.id = COALESCE(
    i.schedule_id,
    (SELECT d.id FROM schedules d WHERE d.workspace_id = i.workspace_id AND d.is_default AND d.deleted_at IS NULL)
)
WHERE i.workspace_id = @workspace_id;

-- name: DeleteReportFacts :exec
-- DeleteReportFacts drops every fact of the conversations, before they
-- are written again or because they no longer count.
WITH replies AS (
    DELETE FROM conversation_reply_facts r WHERE r.workspace_id = @workspace_id AND r.conversation_id = ANY (@ids::uuid[])
), members AS (
    DELETE FROM conversation_member_facts f WHERE f.workspace_id = @workspace_id AND f.conversation_id = ANY (@ids::uuid[])
)
DELETE FROM conversation_facts cf WHERE cf.workspace_id = @workspace_id AND cf.conversation_id = ANY (@ids::uuid[]);

-- name: InsertConversationFacts :exec
-- InsertConversationFacts writes conversation facts given as JSON rows
-- named like the columns.
INSERT INTO conversation_facts
SELECT (s.p).* FROM (SELECT jsonb_populate_recordset(NULL::conversation_facts, @rows::jsonb) AS p) s;

-- name: InsertReplyFacts :exec
-- InsertReplyFacts writes customer turns given as JSON rows.
INSERT INTO conversation_reply_facts
SELECT (s.p).* FROM (SELECT jsonb_populate_recordset(NULL::conversation_reply_facts, @rows::jsonb) AS p) s;

-- name: InsertMemberFacts :exec
-- InsertMemberFacts writes teammates' part in conversations given as
-- JSON rows.
INSERT INTO conversation_member_facts
SELECT (s.p).* FROM (SELECT jsonb_populate_recordset(NULL::conversation_member_facts, @rows::jsonb) AS p) s;

-- name: TakeReportPending :many
-- TakeReportPending removes and returns the conversations due to have
-- their facts worked out again, across workspaces.
SELECT workspace_id::uuid AS workspace_id, conversation_id::uuid AS conversation_id
FROM dimmi_report_take(@batch::integer, @due_before::timestamptz);

-- name: MarkReportPending :exec
-- MarkReportPending queues the conversations to be worked out again.
SELECT dimmi_report_mark(@workspace_id::uuid, id) FROM unnest(@ids::uuid[]) AS id;

-- name: ReportRebuilds :many
-- ReportRebuilds returns workspaces with backfill or rebuild work left.
SELECT workspace_id::uuid AS workspace_id, backfilled::boolean AS backfilled FROM dimmi_report_rebuilds(@batch::integer);

-- name: FindReportState :one
-- FindReportState returns the workspace's report state.
SELECT * FROM report_states WHERE workspace_id = @workspace_id;

-- name: ReportRebuildChunk :many
-- ReportRebuildChunk returns the next conversation ids after the cursor,
-- in id order, for a full rebuild. The rebuild job asks, no member is.
SELECT id FROM conversations
WHERE workspace_id = @workspace_id AND id > @after::uuid
ORDER BY id
LIMIT @batch;

-- name: SetReportRebuildCursor :exec
-- SetReportRebuildCursor records how far a rebuild has come, and ends it
-- when the cursor is null.
UPDATE report_states SET
    rebuild_cursor = sqlc.narg(cursor)::uuid,
    rebuild_requested_at = CASE WHEN sqlc.narg(cursor)::uuid IS NULL THEN NULL ELSE rebuild_requested_at END,
    rebuilt_at = CASE WHEN sqlc.narg(cursor)::uuid IS NULL THEN @now::timestamptz ELSE rebuilt_at END,
    updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id;

-- name: RequestReportRebuild :exec
-- RequestReportRebuild asks for every conversation of the workspace to
-- be worked out again.
INSERT INTO report_states (workspace_id, log_started_at, backfilled_at, rebuild_requested_at)
VALUES (@workspace_id, @now::timestamptz, @now::timestamptz, @now::timestamptz)
ON CONFLICT (workspace_id) DO UPDATE SET rebuild_requested_at = @now::timestamptz, rebuild_cursor = NULL,
    updated_at = @now::timestamptz;

-- name: BackfillConversationEvents :execrows
-- BackfillConversationEvents rebuilds the log of conversations started
-- before it existed, from a chunk of ids: their start, every change
-- their timeline recorded, and a close or hand-off the timeline missed.
-- Only what happened before the log started is written, so nothing the
-- triggers logged is doubled.
WITH chunk AS (
    SELECT c.* FROM conversations c
    WHERE c.workspace_id = @workspace_id AND c.id > @after::uuid AND c.id <= @until::uuid
      AND c.created_at < @log_started_at::timestamptz
), timeline AS (
    SELECT m.conversation_id, m.created_at, m.event ->> 'name' AS name, m.event
    FROM messages m
    JOIN chunk c ON c.id = m.conversation_id
    WHERE m.workspace_id = @workspace_id AND m.type = 'event' AND m.created_at < @log_started_at::timestamptz
      AND m.event ->> 'name' IN (
          'conversation.closed', 'conversation.reopened', 'conversation.snoozed', 'conversation.unsnoozed',
          'conversation.assigned', 'ai.resumed', 'conversation.merged'
      )
), rows AS (
    SELECT c.workspace_id, c.id AS conversation_id, 'created' AS kind, c.created_at AS occurred_at,
        NULL::uuid AS member_id, NULL::uuid AS team_id, NULL::text AS reason
    FROM chunk c
    UNION ALL
    SELECT @workspace_id, t.conversation_id, CASE t.name
            WHEN 'conversation.closed' THEN 'closed'
            WHEN 'conversation.reopened' THEN 'reopened'
            WHEN 'conversation.snoozed' THEN 'snoozed'
            WHEN 'conversation.unsnoozed' THEN 'unsnoozed'
            WHEN 'ai.resumed' THEN 'ai_resumed'
            ELSE 'merged'
        END, t.created_at, NULL::uuid, NULL::uuid, NULL::text
    FROM timeline t
    WHERE t.name <> 'conversation.assigned'
    UNION ALL
    SELECT c.workspace_id, c.id, 'closed', c.closed_at, NULL, NULL, NULL
    FROM chunk c
    WHERE c.status = 'closed' AND c.closed_at IS NOT NULL
      AND NOT EXISTS (SELECT 1 FROM timeline t WHERE t.conversation_id = c.id AND t.name = 'conversation.closed')
    UNION ALL
    SELECT c.workspace_id, c.id, 'assigned', c.created_at, c.assignee_member_id, NULL, NULL
    FROM chunk c
    WHERE c.assignee_member_id IS NOT NULL
      AND NOT EXISTS (SELECT 1 FROM timeline t WHERE t.conversation_id = c.id AND t.name = 'conversation.assigned')
    UNION ALL
    SELECT c.workspace_id, c.id, 'team_assigned', c.created_at, NULL, c.assignee_team_id, NULL
    FROM chunk c
    WHERE c.assignee_team_id IS NOT NULL
      AND NOT EXISTS (SELECT 1 FROM timeline t WHERE t.conversation_id = c.id AND t.name = 'conversation.assigned')
    UNION ALL
    SELECT c.workspace_id, c.id, 'ai_handed_off', c.ai_handed_off_at, NULL, NULL, c.ai_handoff_reason
    FROM chunk c
    WHERE c.ai_handed_off_at IS NOT NULL
    UNION ALL
    SELECT c.workspace_id, c.id, 'merged', c.updated_at, NULL, NULL, NULL
    FROM chunk c
    WHERE c.merged_into_id IS NOT NULL
      AND NOT EXISTS (SELECT 1 FROM timeline t WHERE t.conversation_id = c.id AND t.name = 'conversation.merged')
)
INSERT INTO conversation_events (workspace_id, conversation_id, kind, occurred_at, source, member_id, team_id, reason)
SELECT workspace_id, conversation_id, kind, occurred_at, 'backfill', member_id, team_id, reason
FROM rows
WHERE NOT EXISTS (
    SELECT 1 FROM conversation_events e WHERE e.conversation_id = rows.conversation_id AND e.source = 'backfill'
);

-- name: ReportTimelineAssignments :many
-- ReportTimelineAssignments returns the assignments the timeline of a
-- chunk of conversations recorded before the log started, with the
-- public ids of the member and team taking over. The backfill asks.
SELECT m.conversation_id, m.created_at,
    COALESCE(m.event #>> '{to,member,id}', '')::text AS member,
    COALESCE(m.event #>> '{to,team,id}', '')::text AS team
FROM messages m
JOIN conversations c ON c.id = m.conversation_id
WHERE m.workspace_id = @workspace_id AND c.workspace_id = @workspace_id
  AND c.id > @after::uuid AND c.id <= @until::uuid AND c.created_at < @log_started_at::timestamptz
  AND m.type = 'event' AND m.event ->> 'name' = 'conversation.assigned'
  AND m.created_at < @log_started_at::timestamptz
ORDER BY m.conversation_id, m.created_at;

-- name: InsertBackfillEvents :exec
-- InsertBackfillEvents logs backfilled changes given as JSON rows.
INSERT INTO conversation_events (workspace_id, conversation_id, kind, occurred_at, source, member_id, team_id, reason)
SELECT workspace_id, conversation_id, kind, occurred_at, 'backfill', member_id, team_id, reason
FROM jsonb_to_recordset(@rows::jsonb) AS x(
    workspace_id uuid, conversation_id uuid, kind text, occurred_at timestamptz, member_id uuid, team_id uuid,
    reason text
);

-- name: ReportBackfillBounds :many
-- ReportBackfillBounds returns every thousandth conversation id of the
-- workspace, the bounds of the backfill's chunks. The backfill asks.
SELECT id FROM (
    SELECT id, row_number() OVER (ORDER BY id) AS n FROM conversations WHERE workspace_id = @workspace_id
) x
WHERE n % 1000 = 0
ORDER BY id;

-- name: FinishReportBackfill :exec
-- FinishReportBackfill records that the workspace's log is complete.
UPDATE report_states SET backfilled_at = @now::timestamptz, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id;

-- name: SnapshotBacklog :exec
-- SnapshotBacklog records the conversations waiting at the end of the
-- day, by inbox and team, from the live state, and the day as taken.
INSERT INTO backlog_snapshots (workspace_id, day, inbox_id, team_id, open, snoozed, unassigned,
        under_day, under_three_days, under_week, over_week, taken_at)
    SELECT @workspace_id, CAST(@snapshot_day::text AS date), f.inbox_id, f.assignee_team_id,
        count(*) FILTER (WHERE f.status = 'open'),
        count(*) FILTER (WHERE f.status = 'snoozed'),
        count(*) FILTER (WHERE f.assignee_member_id IS NULL),
        count(*) FILTER (WHERE f.started_at > @day_ago::timestamptz),
        count(*) FILTER (WHERE f.started_at <= @day_ago::timestamptz AND f.started_at > @days_ago::timestamptz),
        count(*) FILTER (WHERE f.started_at <= @days_ago::timestamptz AND f.started_at > @week_ago::timestamptz),
        count(*) FILTER (WHERE f.started_at <= @week_ago::timestamptz),
        @taken_at::timestamptz
    FROM conversation_facts f
    WHERE f.workspace_id = @workspace_id AND f.status <> 'closed'
    GROUP BY f.inbox_id, f.assignee_team_id
    ON CONFLICT (workspace_id, day, inbox_id, team_id) DO NOTHING;

-- name: SetReportSnapshotDay :exec
-- SetReportSnapshotDay records the last day the backlog was counted.
INSERT INTO report_states (workspace_id, log_started_at, backfilled_at, snapshot_day)
VALUES (@workspace_id, @taken_at::timestamptz, @taken_at::timestamptz, CAST(@snapshot_day::text AS date))
ON CONFLICT (workspace_id) DO UPDATE SET snapshot_day = CAST(@snapshot_day::text AS date), updated_at = @taken_at::timestamptz;

-- name: ReportSnapshotsDue :many
-- ReportSnapshotsDue returns the workspaces whose day ended since their
-- last backlog snapshot, with the day to record.
SELECT workspace_id::uuid AS workspace_id, to_char(day, 'YYYY-MM-DD')::text AS day FROM dimmi_report_snapshots_due(@taken_at::timestamptz);

-- name: MarkImportSource :exec
-- MarkImportSource tags the changes the rest of the transaction logs as
-- imported history.
SELECT set_config('dimmi.event_source', 'import', true);
