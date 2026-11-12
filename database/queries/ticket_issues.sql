-- name: CreateTicketIssue :one
-- CreateTicketIssue links an issue or incident to a ticket, refreshing
-- the copy when it was linked already.
INSERT INTO ticket_issues (workspace_id, ticket_id, installation_id, provider, external_id, key, url, title, status,
    status_group, team_name, warning, synced_at, created_by_member_id, created_at, updated_at)
VALUES (@workspace_id, @ticket_id, sqlc.narg(installation_id), @provider, @external_id, @key, @url, @title, @status,
    @status_group, @team_name, sqlc.narg(warning), @now, sqlc.narg(created_by_member_id), @now, @now)
ON CONFLICT (ticket_id, provider, external_id) DO UPDATE SET key = excluded.key, url = excluded.url,
    title = excluded.title, status = excluded.status, status_group = excluded.status_group,
    team_name = excluded.team_name, warning = excluded.warning, synced_at = excluded.synced_at,
    updated_at = excluded.updated_at
RETURNING *;

-- name: FindTicketIssue :one
-- FindTicketIssue returns one link.
SELECT * FROM ticket_issues WHERE workspace_id = @workspace_id AND id = @id;

-- name: DeleteTicketIssue :exec
-- DeleteTicketIssue unlinks an issue; the issue itself stays.
DELETE FROM ticket_issues WHERE workspace_id = @workspace_id AND id = @id;

-- name: ListTicketIssues :many
-- ListTicketIssues returns the issues linked to a ticket.
SELECT * FROM ticket_issues WHERE workspace_id = @workspace_id AND ticket_id = @ticket_id ORDER BY created_at, id;

-- name: ListIssuesOfTickets :many
-- ListIssuesOfTickets returns the issues linked to each of the tickets.
SELECT * FROM ticket_issues WHERE workspace_id = @workspace_id AND ticket_id = ANY (@ticket_ids::uuid[])
ORDER BY created_at, id;

-- name: SyncTicketIssue :one
-- SyncTicketIssue stores what the provider says of an issue now.
UPDATE ticket_issues SET title = @title, status = @status, status_group = @status_group, team_name = @team_name,
    warning = sqlc.narg(warning), sync_error = NULL, synced_at = @now, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: FailTicketIssueSync :exec
-- FailTicketIssueSync records why the last check of an issue failed.
UPDATE ticket_issues SET sync_error = @sync_error, synced_at = @now, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id;

-- name: TicketIssuesByExternalID :many
-- TicketIssuesByExternalID finds every link to a provider's issue across
-- workspaces, for an incoming webhook.
SELECT d.workspace_id::uuid AS workspace_id, d.id::uuid AS id
FROM ticket_issues_by_external_id(@provider::text, @external_id::text) AS d;

-- name: TicketIssuesDueSync :many
-- TicketIssuesDueSync returns the open links whose last check is older
-- than the time, across workspaces.
SELECT d.workspace_id::uuid AS workspace_id, d.id::uuid AS id
FROM ticket_issues_due_sync(@synced_before::timestamptz, @row_limit::integer) AS d;
