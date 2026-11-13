-- name: NextTicketNumber :one
-- NextTicketNumber hands out the workspace's next ticket number.
UPDATE workspaces SET next_ticket_number = next_ticket_number + 1
WHERE id = @workspace_id
RETURNING (next_ticket_number - 1)::bigint AS number;

-- name: CreateTicket :one
-- CreateTicket stores a new ticket.
INSERT INTO tickets (workspace_id, number, kind, type_id, state_id, state_group, conversation_id, contact_id,
    company_id, inbox_id, title, description, fields, suggestions, priority, assignee_member_id, assignee_team_id,
    external_id, sla_policy_id, customer_visible, created_by_type, created_by_id, state_since, resolved_at,
    created_at, updated_at)
VALUES (@workspace_id, @number, @kind, @type_id, @state_id, @state_group, sqlc.narg(conversation_id),
    sqlc.narg(contact_id), sqlc.narg(company_id), @inbox_id, @title, @description, @fields, @suggestions, @priority,
    sqlc.narg(assignee_member_id), sqlc.narg(assignee_team_id), sqlc.narg(external_id), sqlc.narg(sla_policy_id),
    @customer_visible, @created_by_type, sqlc.narg(created_by_id), @now, sqlc.narg(resolved_at), @now, @now)
RETURNING *;

-- name: FindTicket :one
-- FindTicket returns one ticket of the workspace, trashed or not. The
-- caller checks the member may see it.
SELECT * FROM tickets WHERE workspace_id = @workspace_id AND id = @id;

-- name: FindTicketByNumber :one
-- FindTicketByNumber returns the ticket with the reference number. The
-- caller checks the member may see it.
SELECT * FROM tickets WHERE workspace_id = @workspace_id AND number = @number;

-- name: FindTicketByExternalID :one
-- FindTicketByExternalID returns the ticket a client created under its
-- own id, so a retried create returns it.
SELECT * FROM tickets WHERE workspace_id = @workspace_id AND external_id = @external_id;

-- name: FindConversationTicket :one
-- FindConversationTicket returns the customer ticket tied to the
-- conversation.
SELECT * FROM tickets WHERE workspace_id = @workspace_id AND conversation_id = @conversation_id;

-- name: LockTicket :one
-- LockTicket holds a ticket's row until the transaction ends.
SELECT * FROM tickets WHERE workspace_id = @workspace_id AND id = @id FOR UPDATE;

-- name: SaveTicket :one
-- SaveTicket writes every changeable column of a ticket and bumps its
-- version.
UPDATE tickets SET type_id = @type_id, state_id = @state_id, state_group = @state_group, title = @title,
    description = @description, fields = @fields, suggestions = @suggestions, priority = @priority,
    assignee_member_id = sqlc.narg(assignee_member_id), assignee_team_id = sqlc.narg(assignee_team_id),
    inbox_id = @inbox_id, contact_id = sqlc.narg(contact_id), company_id = sqlc.narg(company_id),
    sla_policy_id = sqlc.narg(sla_policy_id), close_loop = @close_loop, state_since = @state_since,
    resolved_at = sqlc.narg(resolved_at), auto_resolve_warned_at = sqlc.narg(auto_resolve_warned_at),
    conversation_id = sqlc.narg(conversation_id), version = version + 1, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: TrashTicket :one
-- TrashTicket moves a ticket to the trash.
UPDATE tickets SET deleted_at = @now, version = version + 1, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL
RETURNING *;

-- name: RestoreTicket :one
-- RestoreTicket takes a ticket out of the trash.
UPDATE tickets SET deleted_at = NULL, version = version + 1, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NOT NULL
RETURNING *;

-- name: FilterTickets :many
-- FilterTickets returns a page of the tickets the member may see,
-- newest first, after the cursor.
SELECT t.* FROM tickets t
WHERE t.workspace_id = @workspace_id
  AND (CASE WHEN @trashed::boolean THEN t.deleted_at IS NOT NULL ELSE t.deleted_at IS NULL END)
  AND dimmi_conversation_visible(
      t.assignee_member_id, t.assignee_team_id, t.inbox_id, @access_mode::text,
      sqlc.narg(access_viewer)::uuid, @access_teams::uuid[], @access_unassigned::boolean,
      @access_hidden_inboxes::uuid[]
  )
  AND (cardinality(@type_ids::uuid[]) = 0 OR t.type_id = ANY (@type_ids::uuid[]))
  AND (cardinality(@kinds::text[]) = 0 OR t.kind = ANY (@kinds::text[]))
  AND (cardinality(@state_ids::uuid[]) = 0 OR t.state_id = ANY (@state_ids::uuid[]))
  AND (cardinality(@groups::text[]) = 0 OR t.state_group = ANY (@groups::text[]))
  AND (sqlc.narg(assignee)::uuid IS NULL OR t.assignee_member_id = sqlc.narg(assignee)::uuid)
  AND (NOT @unassigned::boolean OR t.assignee_member_id IS NULL)
  AND (sqlc.narg(team)::uuid IS NULL OR t.assignee_team_id = sqlc.narg(team)::uuid)
  AND (sqlc.narg(company)::uuid IS NULL OR t.company_id = sqlc.narg(company)::uuid)
  AND (sqlc.narg(contact)::uuid IS NULL OR t.contact_id = sqlc.narg(contact)::uuid)
  AND (sqlc.narg(number)::bigint IS NULL OR t.number = sqlc.narg(number)::bigint)
  AND (sqlc.narg(external_id)::text IS NULL OR t.external_id = sqlc.narg(external_id)::text)
  AND (sqlc.narg(priority)::text IS NULL OR t.priority = sqlc.narg(priority)::text)
  AND t.fields @> @fields::jsonb
  AND (sqlc.narg(issue_group)::text IS NULL OR EXISTS (
      SELECT 1 FROM ticket_issues i WHERE i.ticket_id = t.id AND i.status_group = sqlc.narg(issue_group)::text))
  AND (sqlc.narg(created_from)::timestamptz IS NULL OR t.created_at >= sqlc.narg(created_from)::timestamptz)
  AND (sqlc.narg(created_to)::timestamptz IS NULL OR t.created_at < sqlc.narg(created_to)::timestamptz)
  AND (sqlc.narg(resolved_from)::timestamptz IS NULL OR t.resolved_at >= sqlc.narg(resolved_from)::timestamptz)
  AND (sqlc.narg(resolved_to)::timestamptz IS NULL OR t.resolved_at < sqlc.narg(resolved_to)::timestamptz)
  AND (sqlc.narg(query)::text IS NULL OR t.title ILIKE '%' || sqlc.narg(query)::text || '%'
      OR t.number::text = sqlc.narg(query)::text)
  AND (sqlc.narg(linked_conversation)::uuid IS NULL OR EXISTS (
      SELECT 1 FROM ticket_links l WHERE l.from_ticket_id = t.id
        AND l.conversation_id = sqlc.narg(linked_conversation)::uuid))
  AND (sqlc.narg(after_created)::timestamptz IS NULL
      OR (t.created_at, t.id) < (sqlc.narg(after_created)::timestamptz, sqlc.narg(after_id)::uuid))
ORDER BY t.created_at DESC, t.id DESC
LIMIT @row_limit;

-- name: CountTicketQueues :many
-- CountTicketQueues counts the live tickets the member may see by type
-- and state group, for the inbox sidebar.
SELECT t.type_id, t.state_group, count(*) AS tickets
FROM tickets t
WHERE t.workspace_id = @workspace_id AND t.deleted_at IS NULL
  AND dimmi_conversation_visible(
      t.assignee_member_id, t.assignee_team_id, t.inbox_id, @access_mode::text,
      sqlc.narg(access_viewer)::uuid, @access_teams::uuid[], @access_unassigned::boolean,
      @access_hidden_inboxes::uuid[]
  )
GROUP BY t.type_id, t.state_group;

-- name: ListTicketsByIDs :many
-- ListTicketsByIDs returns the tickets with the ids. The caller checks
-- the member may see them.
SELECT * FROM tickets WHERE workspace_id = @workspace_id AND id = ANY (@ids::uuid[]);

-- name: ListConversationTickets :many
-- ListConversationTickets returns, for each of the conversations, its
-- own customer ticket.
SELECT * FROM tickets
WHERE workspace_id = @workspace_id AND conversation_id = ANY (@conversation_ids::uuid[]) AND deleted_at IS NULL;

-- name: ListContactTickets :many
-- ListContactTickets returns a customer's own customer-visible tickets,
-- or those of their company when the type shares them, newest first.
SELECT t.* FROM tickets t
JOIN ticket_types ty ON ty.id = t.type_id
WHERE t.workspace_id = @workspace_id AND t.deleted_at IS NULL AND t.kind = 'customer' AND t.customer_visible
  AND (t.contact_id = @contact_id
      OR (@company_scope::boolean AND ty.company_sharing AND t.company_id IS NOT NULL
          AND t.company_id = sqlc.narg(company_id)::uuid))
  AND (NOT @open_only::boolean OR t.state_group <> 'resolved')
ORDER BY t.created_at DESC
LIMIT @row_limit;

-- name: SetTicketSuggestions :exec
-- SetTicketSuggestions stores the field values the AI proposed.
UPDATE tickets SET suggestions = @suggestions, updated_at = @now WHERE workspace_id = @workspace_id AND id = @id;

-- name: MarkTicketAutoResolveWarned :exec
-- MarkTicketAutoResolveWarned remembers the customer was told the
-- ticket will resolve itself.
UPDATE tickets SET auto_resolve_warned_at = @now WHERE workspace_id = @workspace_id AND id = @id;

-- name: MarkTicketNotified :exec
-- MarkTicketNotified remembers the state the customer was last told of.
UPDATE tickets SET notified_state_id = @state_id WHERE workspace_id = @workspace_id AND id = @id;

-- name: ListRecentConversationText :many
-- ListRecentConversationText returns the newest messages of a
-- conversation with text, for the AI to read.
SELECT author_type, body::text AS body FROM messages
WHERE workspace_id = @workspace_id AND conversation_id = @conversation_id AND type = 'message' AND body IS NOT NULL
ORDER BY created_at DESC
LIMIT 40;

-- name: TicketsDueAutoResolve :many
-- TicketsDueAutoResolve returns the tickets across workspaces whose
-- wait on the customer ends within a day.
SELECT d.workspace_id::uuid AS workspace_id, d.id::uuid AS id
FROM tickets_due_auto_resolve(@due_at::timestamptz, @row_limit::integer) AS d;
