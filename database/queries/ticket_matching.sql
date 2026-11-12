-- name: ListOpenTrackers :many
-- ListOpenTrackers returns the workspace's trackers not yet resolved,
-- most recently changed first, for matching a new conversation. The
-- system matches; no member is asking.
SELECT * FROM tickets
WHERE workspace_id = @workspace_id AND kind = 'tracker' AND state_group <> 'resolved' AND deleted_at IS NULL
ORDER BY updated_at DESC
LIMIT 50;

-- name: CreateTrackerSuggestion :exec
-- CreateTrackerSuggestion keeps the AI's match of a conversation to a
-- tracker, once.
INSERT INTO tracker_suggestions (workspace_id, conversation_id, ticket_id, rank, status, created_at, updated_at)
VALUES (@workspace_id, @conversation_id, @ticket_id, @rank, @status, @now, @now)
ON CONFLICT (conversation_id, ticket_id) DO NOTHING;

-- name: ListTrackerSuggestions :many
-- ListTrackerSuggestions returns the open trackers suggested for the
-- conversation that it is not linked to yet, best first, with how many
-- conversations each already reaches.
SELECT s.id AS suggestion_id, s.rank, s.created_at AS suggested_at, t.*,
    (SELECT count(*) FROM ticket_links l WHERE l.from_ticket_id = t.id AND l.conversation_id IS NOT NULL)::bigint
        AS linked_count
FROM tracker_suggestions s
JOIN tickets t ON t.id = s.ticket_id
WHERE s.workspace_id = @workspace_id AND s.conversation_id = @conversation_id AND s.status = 'open'
  AND t.deleted_at IS NULL AND t.state_group <> 'resolved'
  AND NOT EXISTS (SELECT 1 FROM ticket_links l WHERE l.from_ticket_id = t.id AND l.conversation_id = s.conversation_id)
ORDER BY s.rank, s.created_at;

-- name: ListTrackerReports :many
-- ListTrackerReports returns what the customers linked to a tracker
-- wrote first and their company's plan, newest link first, for the
-- tracker's summary.
SELECT c.id AS conversation_id, c.subject, coalesce(co.custom_attributes ->> 'plan', '')::text AS company_plan,
    (SELECT m.body FROM messages m WHERE m.conversation_id = c.id AND m.author_type = 'contact'
     ORDER BY m.created_at LIMIT 1)::text AS first_message
FROM ticket_links l
JOIN conversations c ON c.id = l.conversation_id
JOIN contacts ct ON ct.id = c.contact_id
LEFT JOIN companies co ON co.id = ct.company_id
WHERE l.workspace_id = @workspace_id AND l.from_ticket_id = @ticket_id AND c.deleted_at IS NULL
ORDER BY l.created_at DESC
LIMIT 60;

-- name: FindTicketSummary :one
-- FindTicketSummary returns a tracker's kept summary.
SELECT * FROM ticket_summaries WHERE workspace_id = @workspace_id AND ticket_id = @ticket_id;

-- name: SaveTicketSummary :exec
-- SaveTicketSummary keeps a tracker's summary with the reports it read.
INSERT INTO ticket_summaries (ticket_id, workspace_id, summary, report_count, generated_at)
VALUES (@ticket_id, @workspace_id, @summary, @report_count, @now)
ON CONFLICT (ticket_id) DO UPDATE SET summary = excluded.summary, report_count = excluded.report_count,
    generated_at = excluded.generated_at;
