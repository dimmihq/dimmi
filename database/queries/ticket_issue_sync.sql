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

-- name: CountTicketImpact :one
-- CountTicketImpact counts what a ticket reaches for the issues linked
-- to it: its own conversation, customer and company, and those of the
-- conversations linked to it as a tracker, with the companies' monthly
-- revenue.
WITH convs AS (
    SELECT c.id, c.contact_id FROM ticket_links l
    JOIN conversations c ON c.id = l.conversation_id
    WHERE l.workspace_id = @workspace_id AND l.from_ticket_id = @ticket_id AND c.deleted_at IS NULL
    UNION
    SELECT c.id, c.contact_id FROM tickets t
    JOIN conversations c ON c.id = t.conversation_id
    WHERE t.workspace_id = @workspace_id AND t.id = @ticket_id AND c.deleted_at IS NULL
), people AS (
    SELECT contact_id AS id FROM convs WHERE contact_id IS NOT NULL
    UNION
    SELECT contact_id FROM tickets WHERE workspace_id = @workspace_id AND id = @ticket_id AND contact_id IS NOT NULL
), firms AS (
    SELECT ct.company_id AS id FROM people p JOIN contacts ct ON ct.id = p.id WHERE ct.company_id IS NOT NULL
    UNION
    SELECT company_id FROM tickets WHERE workspace_id = @workspace_id AND id = @ticket_id AND company_id IS NOT NULL
)
SELECT (SELECT count(*) FROM convs)::bigint AS conversations, (SELECT count(*) FROM people)::bigint AS customers,
    (SELECT count(*) FROM firms)::bigint AS companies,
    coalesce((SELECT sum(dimmi_company_revenue(co.custom_attributes)) FROM companies co
        WHERE co.workspace_id = @workspace_id AND co.id IN (SELECT id FROM firms)), 0)::float8 AS revenue;

-- name: TicketImpactCompanies :many
-- TicketImpactCompanies names the companies a ticket reaches, as
-- CountTicketImpact counts them, the largest by revenue first.
WITH people AS (
    SELECT c.contact_id AS id FROM ticket_links l
    JOIN conversations c ON c.id = l.conversation_id
    WHERE l.workspace_id = @workspace_id AND l.from_ticket_id = @ticket_id AND c.deleted_at IS NULL
    UNION
    SELECT c.contact_id FROM tickets t
    JOIN conversations c ON c.id = t.conversation_id
    WHERE t.workspace_id = @workspace_id AND t.id = @ticket_id AND c.deleted_at IS NULL
    UNION
    SELECT contact_id FROM tickets WHERE workspace_id = @workspace_id AND id = @ticket_id
), firms AS (
    SELECT ct.company_id AS id FROM people p JOIN contacts ct ON ct.id = p.id WHERE ct.company_id IS NOT NULL
    UNION
    SELECT company_id FROM tickets WHERE workspace_id = @workspace_id AND id = @ticket_id AND company_id IS NOT NULL
)
SELECT co.id, co.name, dimmi_company_revenue(co.custom_attributes)::float8 AS revenue
FROM companies co
WHERE co.workspace_id = @workspace_id AND co.id IN (SELECT id FROM firms)
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
