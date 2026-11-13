-- name: CreateTicketLink :one
-- CreateTicketLink stores a link, or returns nothing when it exists.
INSERT INTO ticket_links (workspace_id, kind, from_ticket_id, to_ticket_id, conversation_id, created_by_member_id,
    created_at)
VALUES (@workspace_id, @kind, @from_ticket_id, sqlc.narg(to_ticket_id), sqlc.narg(conversation_id),
    sqlc.narg(created_by_member_id), @now)
ON CONFLICT DO NOTHING
RETURNING *;

-- name: FindTicketLink :one
-- FindTicketLink returns one link.
SELECT * FROM ticket_links WHERE workspace_id = @workspace_id AND id = @id;

-- name: DeleteTicketLink :exec
-- DeleteTicketLink removes a link.
DELETE FROM ticket_links WHERE workspace_id = @workspace_id AND id = @id;

-- name: ListTicketLinks :many
-- ListTicketLinks returns every link from or to the ticket, and the
-- links to its conversation, oldest first.
SELECT * FROM ticket_links
WHERE workspace_id = @workspace_id
  AND (from_ticket_id = @ticket_id OR to_ticket_id = @ticket_id
      OR (sqlc.narg(conversation_id)::uuid IS NOT NULL AND conversation_id = sqlc.narg(conversation_id)::uuid))
ORDER BY created_at, id;

-- name: ListConversationLinks :many
-- ListConversationLinks returns the trackers the conversation is linked
-- to.
SELECT * FROM ticket_links WHERE workspace_id = @workspace_id AND conversation_id = @conversation_id
ORDER BY created_at, id;

-- name: ListLinksOfConversations :many
-- ListLinksOfConversations returns the tracker links of the
-- conversations, for their rows in the inbox.
SELECT * FROM ticket_links
WHERE workspace_id = @workspace_id AND conversation_id = ANY (@conversation_ids::uuid[]);

-- name: ListTrackerConversations :many
-- ListTrackerConversations returns a tracker's linked conversations with
-- their customer, company and the customer ticket's state, newest link
-- first, after the cursor. The caller checks the member may read the
-- tracker; a linked conversation is named to anyone who may.
SELECT l.id AS link_id, l.created_at AS linked_at, c.id AS conversation_id, c.number AS conversation_number,
    c.subject, c.status, c.last_message_at, c.channel_connection_id,
    ct.id AS contact_id, ct.name AS contact_name, ct.email AS contact_email,
    co.id AS company_id, co.name AS company_name, coalesce(co.custom_attributes ->> 'plan', '')::text AS company_plan,
    dimmi_company_revenue(co.custom_attributes)::float8 AS company_revenue,
    tk.id AS ticket_id, tk.number AS ticket_number, tk.state_id AS ticket_state_id
FROM ticket_links l
JOIN conversations c ON c.id = l.conversation_id
JOIN contacts ct ON ct.id = c.contact_id
LEFT JOIN companies co ON co.id = ct.company_id
LEFT JOIN tickets tk ON tk.conversation_id = c.id AND tk.deleted_at IS NULL
WHERE l.workspace_id = @workspace_id AND l.from_ticket_id = @ticket_id AND l.conversation_id IS NOT NULL
  AND c.deleted_at IS NULL
  AND (sqlc.narg(after_created)::timestamptz IS NULL
      OR (l.created_at, l.id) < (sqlc.narg(after_created)::timestamptz, sqlc.narg(after_id)::uuid))
ORDER BY l.created_at DESC, l.id DESC
LIMIT @row_limit;

-- name: CountTrackerImpact :one
-- CountTrackerImpact counts what a tracker reaches: linked
-- conversations, distinct customers and companies, and the companies'
-- monthly revenue.
SELECT count(DISTINCT c.id) AS conversations, count(DISTINCT c.contact_id) AS customers,
    count(DISTINCT ct.company_id) AS companies,
    coalesce((SELECT sum(dimmi_company_revenue(co.custom_attributes)) FROM companies co WHERE co.id IN (
        SELECT DISTINCT ct2.company_id FROM ticket_links l2
        JOIN conversations c2 ON c2.id = l2.conversation_id
        JOIN contacts ct2 ON ct2.id = c2.contact_id
        WHERE l2.from_ticket_id = @ticket_id AND c2.deleted_at IS NULL)), 0)::float8 AS revenue
FROM ticket_links l
JOIN conversations c ON c.id = l.conversation_id
JOIN contacts ct ON ct.id = c.contact_id
WHERE l.workspace_id = @workspace_id AND l.from_ticket_id = @ticket_id AND c.deleted_at IS NULL;

-- name: CountLinkedConversations :many
-- CountLinkedConversations counts the linked conversations of each
-- tracker.
SELECT from_ticket_id, count(*) AS conversations
FROM ticket_links
WHERE workspace_id = @workspace_id AND from_ticket_id = ANY (@ticket_ids::uuid[]) AND conversation_id IS NOT NULL
GROUP BY from_ticket_id;

-- name: ListTrackerRecipients :many
-- ListTrackerRecipients returns every conversation linked to a tracker
-- that a mass update reaches, with when the customer and the team last
-- wrote. The system sends it, no member is asking.
SELECT c.id, c.contact_id, c.status, c.last_customer_message_at, c.last_reply_at, l.created_at AS linked_at
FROM ticket_links l
JOIN conversations c ON c.id = l.conversation_id
WHERE l.workspace_id = @workspace_id AND l.from_ticket_id = @ticket_id AND c.deleted_at IS NULL
ORDER BY l.created_at;
