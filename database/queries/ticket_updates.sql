-- name: CreateTicketUpdate :one
-- CreateTicketUpdate stores a mass message about to go to every linked
-- customer.
INSERT INTO ticket_updates (workspace_id, ticket_id, body, resolve, recipients, held, author_member_id, created_at)
VALUES (@workspace_id, @ticket_id, @body, @resolve, @recipients, @held, sqlc.narg(author_member_id), @now)
RETURNING *;

-- name: FindTicketUpdate :one
-- FindTicketUpdate returns one mass message.
SELECT * FROM ticket_updates WHERE workspace_id = @workspace_id AND id = @id;

-- name: ListTicketUpdates :many
-- ListTicketUpdates returns a ticket's mass messages, newest first.
SELECT * FROM ticket_updates WHERE workspace_id = @workspace_id AND ticket_id = @ticket_id
ORDER BY created_at DESC
LIMIT 50;

-- name: CreateTicketUpdateDelivery :exec
-- CreateTicketUpdateDelivery stores one recipient of a mass message,
-- pending or held with the reason.
INSERT INTO ticket_update_deliveries (workspace_id, update_id, conversation_id, status, reason, created_at, updated_at)
VALUES (@workspace_id, @update_id, @conversation_id, @status, sqlc.narg(reason), @now, @now)
ON CONFLICT (update_id, conversation_id) DO NOTHING;

-- name: ListTicketUpdateDeliveries :many
-- ListTicketUpdateDeliveries returns the recipients of mass messages,
-- with each customer's name and address.
SELECT d.*, c.number AS conversation_number, ct.name AS contact_name, ct.email AS contact_email
FROM ticket_update_deliveries d
JOIN conversations c ON c.id = d.conversation_id
JOIN contacts ct ON ct.id = c.contact_id
WHERE d.workspace_id = @workspace_id AND d.update_id = ANY (@update_ids::uuid[])
ORDER BY d.created_at, d.id;

-- name: ListPendingTicketUpdateDeliveries :many
-- ListPendingTicketUpdateDeliveries returns the recipients of a mass
-- message still waiting to be sent.
SELECT * FROM ticket_update_deliveries
WHERE workspace_id = @workspace_id AND update_id = @update_id AND status = 'pending'
ORDER BY created_at, id
LIMIT @row_limit;

-- name: SetTicketUpdateDelivery :exec
-- SetTicketUpdateDelivery records how sending to one recipient went.
UPDATE ticket_update_deliveries SET status = @status, reason = sqlc.narg(reason), attempts = attempts + 1,
    message_id = sqlc.narg(message_id), sent_at = sqlc.narg(sent_at), updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id;

-- name: FinishTicketUpdate :one
-- FinishTicketUpdate counts how a mass message went and marks it done.
UPDATE ticket_updates u SET
    delivered = (SELECT count(*) FROM ticket_update_deliveries d WHERE d.update_id = u.id AND d.status = 'sent'),
    held = (SELECT count(*) FROM ticket_update_deliveries d WHERE d.update_id = u.id AND d.status = 'held'),
    failed = (SELECT count(*) FROM ticket_update_deliveries d WHERE d.update_id = u.id AND d.status = 'failed'),
    status = @status, finished_at = @now
WHERE u.workspace_id = @workspace_id AND u.id = @id
RETURNING *;

-- name: MarkTicketUpdateSending :exec
-- MarkTicketUpdateSending says a mass message's job started.
UPDATE ticket_updates SET status = 'sending' WHERE workspace_id = @workspace_id AND id = @id AND status = 'queued';

-- name: CreateTicketPublicUpdate :one
-- CreateTicketPublicUpdate posts to an incident's public update log.
INSERT INTO ticket_public_updates (workspace_id, ticket_id, body, state_id, update_id, author_member_id, source,
    created_at)
VALUES (@workspace_id, @ticket_id, @body, sqlc.narg(state_id), sqlc.narg(update_id), sqlc.narg(author_member_id),
    @source, @now)
RETURNING *;

-- name: ListTicketPublicUpdates :many
-- ListTicketPublicUpdates returns an incident's public update log,
-- newest first.
SELECT * FROM ticket_public_updates WHERE workspace_id = @workspace_id AND ticket_id = @ticket_id
ORDER BY created_at DESC, id DESC
LIMIT 100;

-- name: ListLatestPublicUpdates :many
-- ListLatestPublicUpdates returns the newest public post of each of the
-- incidents.
SELECT DISTINCT ON (ticket_id) * FROM ticket_public_updates
WHERE workspace_id = @workspace_id AND ticket_id = ANY (@ticket_ids::uuid[])
ORDER BY ticket_id, created_at DESC, id DESC;
