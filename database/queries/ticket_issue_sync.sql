-- name: SyncTicketIssueDetails :one
-- SyncTicketIssueDetails stores what the provider says of an issue now,
-- with its team, severity and when it was first seen done.
UPDATE ticket_issues SET title = @title, status = @status, status_group = @status_group, team_name = @team_name,
    team_id = @team_id, severity = @severity, warning = sqlc.narg(warning), sync_error = NULL, synced_at = @now::timestamptz,
    done_at = CASE WHEN @status_group::text = 'done' THEN coalesce(done_at, @now) ELSE NULL END,
    updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: WarnTicketIssue :exec
-- WarnTicketIssue records a warning on a link, such as a team the app
-- cannot see, and when it was checked.
UPDATE ticket_issues SET warning = @warning, synced_at = @now::timestamptz, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id;

-- name: SetTicketIssueImpact :exec
-- SetTicketIssueImpact records the impact last written to the issue.
UPDATE ticket_issues SET impact_hash = @impact_hash WHERE workspace_id = @workspace_id AND id = @id;

-- name: TicketIssuesOfExternal :many
-- TicketIssuesOfExternal returns the workspace's links to one provider
-- issue.
SELECT * FROM ticket_issues WHERE workspace_id = @workspace_id AND provider = @provider AND external_id = @external_id
ORDER BY created_at, id;

-- name: TicketIssuesAwaitingDeploy :many
-- TicketIssuesAwaitingDeploy returns the workspace's done issues whose
-- fix no deployment has reached yet, done before the time.
SELECT * FROM ticket_issues
WHERE workspace_id = @workspace_id AND done_at IS NOT NULL AND deployed_at IS NULL AND done_at <= @before::timestamptz
ORDER BY done_at, id;

-- name: MarkTicketIssueDeployed :exec
-- MarkTicketIssueDeployed records that the issue's fix reached
-- production.
UPDATE ticket_issues SET deployed_at = @now::timestamptz, updated_at = @now WHERE workspace_id = @workspace_id AND id = @id;

-- name: TicketImpactCompanies :many
-- TicketImpactCompanies names the companies a tracker or incident
-- reaches, the largest by revenue first.
SELECT DISTINCT co.id, co.name, dimmi_company_revenue(co.custom_attributes)::float8 AS revenue
FROM ticket_links l
JOIN conversations c ON c.id = l.conversation_id
JOIN contacts ct ON ct.id = c.contact_id
JOIN companies co ON co.id = ct.company_id
WHERE l.workspace_id = @workspace_id AND l.from_ticket_id = @ticket_id AND c.deleted_at IS NULL
ORDER BY revenue DESC, co.name
LIMIT 50;

-- name: InstallationOwner :one
-- InstallationOwner names the workspace and provider of an installation
-- a delivery is addressed to, before any workspace is known.
SELECT d.workspace_id::uuid AS workspace_id, d.provider::text AS provider, d.status::text AS status
FROM integration_installation_owner(@installation_id::uuid) AS d;

-- name: MergeIntegrationMetadata :one
-- MergeIntegrationMetadata adds keys to an installation's metadata.
UPDATE integration_installations SET metadata = metadata || @metadata::jsonb, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL
RETURNING *;

-- name: CreateTicketUpdateDraft :one
-- CreateTicketUpdateDraft stores a drafted message to everyone linked.
INSERT INTO ticket_update_drafts (workspace_id, ticket_id, issue_id, source, reason, body, author_name, status, resolve,
    created_at, updated_at)
VALUES (@workspace_id, @ticket_id, sqlc.narg(issue_id), @source, @reason, @body, @author_name, @status, @resolve,
    @now, @now)
RETURNING *;

-- name: TicketUpdateDrafts :many
-- TicketUpdateDrafts returns a ticket's drafts, the newest first.
SELECT * FROM ticket_update_drafts WHERE workspace_id = @workspace_id AND ticket_id = @ticket_id
ORDER BY created_at DESC, id DESC
LIMIT 50;

-- name: TicketUpdateDraft :one
-- TicketUpdateDraft returns one draft, locked for a decision.
SELECT * FROM ticket_update_drafts WHERE workspace_id = @workspace_id AND id = @id FOR UPDATE;

-- name: OpenDraftOfTicket :one
-- OpenDraftOfTicket returns the ticket's draft still waiting, of the
-- reason, so a repeated signal refreshes it instead of adding another.
SELECT * FROM ticket_update_drafts
WHERE workspace_id = @workspace_id AND ticket_id = @ticket_id AND reason = @reason
  AND status IN ('waiting_deploy', 'ready')
ORDER BY created_at DESC
LIMIT 1;

-- name: DraftsWaitingDeploy :many
-- DraftsWaitingDeploy returns the drafts of the issues waiting for
-- their fix to ship.
SELECT * FROM ticket_update_drafts
WHERE workspace_id = @workspace_id AND status = 'waiting_deploy' AND issue_id = ANY (@issue_ids::uuid[]);

-- name: DecideTicketUpdateDraft :one
-- DecideTicketUpdateDraft moves a draft on: ready once the fix shipped,
-- sent with the update it became, or discarded.
UPDATE ticket_update_drafts SET status = @status, body = @body, update_id = sqlc.narg(update_id),
    decided_by_member_id = sqlc.narg(decided_by_member_id), decided_at = sqlc.narg(decided_at), updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;
