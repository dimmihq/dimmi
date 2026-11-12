-- name: LockTicketSetup :exec
-- LockTicketSetup holds the workspace's ticket setup until the
-- transaction ends, so two first reads don't both add the defaults.
SELECT pg_advisory_xact_lock(hashtext('ticket_setup:' || @workspace_id::text));

-- name: ListTicketStates :many
-- ListTicketStates returns the workspace's ticket states, archived ones
-- included, group by group in the workspace's order.
SELECT * FROM ticket_states WHERE workspace_id = @workspace_id
ORDER BY CASE state_group
    WHEN 'submitted' THEN 0 WHEN 'in_progress' THEN 1 WHEN 'waiting_customer' THEN 2
    WHEN 'waiting_team' THEN 3 ELSE 4 END, position, created_at, id;

-- name: FindTicketState :one
-- FindTicketState returns one state of the workspace.
SELECT * FROM ticket_states WHERE workspace_id = @workspace_id AND id = @id;

-- name: CreateTicketState :one
-- CreateTicketState stores a state after the others of its group.
INSERT INTO ticket_states (workspace_id, state_group, name, customer_label, notify_customer, purpose, position,
    created_at, updated_at)
VALUES (@workspace_id, @state_group, @name, @customer_label, @notify_customer, sqlc.narg(purpose),
    (SELECT coalesce(max(position) + 1, 0) FROM ticket_states
     WHERE workspace_id = @workspace_id AND state_group = @state_group), @now, @now)
RETURNING *;

-- name: UpdateTicketState :one
-- UpdateTicketState renames a state, changes its customer label or
-- whether moving into it tells the customer.
UPDATE ticket_states SET name = @name, customer_label = @customer_label, notify_customer = @notify_customer,
    updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: ArchiveTicketState :exec
-- ArchiveTicketState hides a state from new work.
UPDATE ticket_states SET archived_at = @now, purpose = NULL, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id;

-- name: SaveTicketStateOrder :exec
-- SaveTicketStateOrder stores the states in the order given; each keeps
-- its group.
UPDATE ticket_states s SET position = (o.ord - 1)::integer, updated_at = @now
FROM unnest(@ids::uuid[]) WITH ORDINALITY AS o(id, ord)
WHERE s.workspace_id = @workspace_id AND s.id = o.id;

-- name: MoveTicketsToState :execrows
-- MoveTicketsToState moves every ticket in a state, trashed ones
-- included, to another state of the same group.
UPDATE tickets SET state_id = @target, version = version + 1, updated_at = @now
WHERE workspace_id = @workspace_id AND state_id = @source;

-- name: ReplaceTypeState :exec
-- ReplaceTypeState lets the types that allowed a removed state use its
-- replacement instead, keeping their defaults.
INSERT INTO ticket_type_states (type_id, state_id, workspace_id, is_default, is_default_resolved)
SELECT ts.type_id, @target, ts.workspace_id, ts.is_default, ts.is_default_resolved FROM ticket_type_states ts
WHERE ts.workspace_id = @workspace_id AND ts.state_id = @source
ON CONFLICT (type_id, state_id) DO UPDATE SET
    is_default = ticket_type_states.is_default OR excluded.is_default,
    is_default_resolved = ticket_type_states.is_default_resolved OR excluded.is_default_resolved;

-- name: DeleteTypeStatesOfState :exec
-- DeleteTypeStatesOfState takes a removed state off every type.
DELETE FROM ticket_type_states WHERE workspace_id = @workspace_id AND state_id = @state_id;

-- name: ListTicketTypes :many
-- ListTicketTypes returns the workspace's ticket types in its order,
-- archived ones last.
SELECT * FROM ticket_types WHERE workspace_id = @workspace_id
ORDER BY archived_at IS NOT NULL, position, created_at, id;

-- name: FindTicketType :one
-- FindTicketType returns one type of the workspace.
SELECT * FROM ticket_types WHERE workspace_id = @workspace_id AND id = @id;

-- name: CountTicketTypes :one
-- CountTicketTypes says how many types the workspace has, archived ones
-- included, so the defaults are added only once.
SELECT count(*) FROM ticket_types WHERE workspace_id = @workspace_id;

-- name: CreateTicketType :one
-- CreateTicketType stores a type after the others.
INSERT INTO ticket_types (workspace_id, kind, name, icon, description, inbox_id, team_id, customer_visible,
    company_sharing, reopen_days, auto_resolve_days, auto_resolve_message,
    ai_auto_fill, position, created_at, updated_at)
VALUES (@workspace_id, @kind, @name, @icon, @description, sqlc.narg(inbox_id), sqlc.narg(team_id), @customer_visible,
    @company_sharing, @reopen_days, sqlc.narg(auto_resolve_days), sqlc.narg(auto_resolve_message), @ai_auto_fill,
    (SELECT coalesce(max(position) + 1, 0) FROM ticket_types WHERE workspace_id = @workspace_id), @now, @now)
RETURNING *;

-- name: UpdateTicketType :one
-- UpdateTicketType changes a type's settings; its kind stays.
UPDATE ticket_types SET name = @name, icon = @icon, description = @description, inbox_id = sqlc.narg(inbox_id),
    team_id = sqlc.narg(team_id), customer_visible = @customer_visible, company_sharing = @company_sharing,
    reopen_days = @reopen_days, auto_resolve_days = sqlc.narg(auto_resolve_days),
    auto_resolve_message = sqlc.narg(auto_resolve_message), ai_auto_fill = @ai_auto_fill, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: ArchiveTicketType :exec
-- ArchiveTicketType hides a type from new tickets; its tickets keep it.
UPDATE ticket_types SET archived_at = @now, updated_at = @now WHERE workspace_id = @workspace_id AND id = @id;

-- name: ListTypeStates :many
-- ListTypeStates returns which states each type of the workspace allows.
SELECT * FROM ticket_type_states WHERE workspace_id = @workspace_id;

-- name: DeleteTypeStates :exec
-- DeleteTypeStates clears the states a type allows before they are
-- saved again.
DELETE FROM ticket_type_states WHERE workspace_id = @workspace_id AND type_id = @type_id;

-- name: AddTypeState :exec
-- AddTypeState lets a type use a state.
INSERT INTO ticket_type_states (type_id, state_id, workspace_id, is_default, is_default_resolved)
VALUES (@type_id, @state_id, @workspace_id, @is_default, @is_default_resolved)
ON CONFLICT (type_id, state_id) DO UPDATE SET is_default = excluded.is_default,
    is_default_resolved = excluded.is_default_resolved;

-- name: CountTicketsByType :many
-- CountTicketsByType counts each type's live tickets, open and in all.
SELECT type_id, count(*) AS total, count(*) FILTER (WHERE state_group <> 'resolved') AS open
FROM tickets WHERE workspace_id = @workspace_id AND deleted_at IS NULL
GROUP BY type_id;

-- name: CountTicketsInState :one
-- CountTicketsInState says how many tickets, trashed ones included, are
-- in a state.
SELECT count(*) FROM tickets WHERE workspace_id = @workspace_id AND state_id = @state_id;

-- name: ListTicketFields :many
-- ListTicketFields returns every field of the workspace's types in their
-- order, archived ones included.
SELECT * FROM ticket_fields WHERE workspace_id = @workspace_id ORDER BY type_id, position, created_at, id;

-- name: ListTypeFields :many
-- ListTypeFields returns one type's fields in their order.
SELECT * FROM ticket_fields WHERE workspace_id = @workspace_id AND type_id = @type_id
ORDER BY position, created_at, id;

-- name: FindTicketField :one
-- FindTicketField returns one field.
SELECT * FROM ticket_fields WHERE workspace_id = @workspace_id AND id = @id;

-- name: CreateTicketField :one
-- CreateTicketField stores a field after the type's others.
INSERT INTO ticket_fields (workspace_id, type_id, key, label, description, data_type, options, currency,
    required_create, required_create_customer, required_resolve, visible_customer, editable_customer, show_when,
    ai_hint, ai_auto, position, created_at, updated_at)
VALUES (@workspace_id, @type_id, @key, @label, @description, @data_type, @options, sqlc.narg(currency),
    @required_create, @required_create_customer, @required_resolve, @visible_customer, @editable_customer,
    sqlc.narg(show_when), sqlc.narg(ai_hint), @ai_auto,
    (SELECT coalesce(max(position) + 1, 0) FROM ticket_fields WHERE workspace_id = @workspace_id AND type_id = @type_id),
    @now, @now)
ON CONFLICT (type_id, key) DO NOTHING
RETURNING *;

-- name: UpdateTicketField :one
-- UpdateTicketField changes a field's settings; its key and data type
-- stay.
UPDATE ticket_fields SET label = @label, description = @description, options = @options,
    currency = sqlc.narg(currency), required_create = @required_create,
    required_create_customer = @required_create_customer, required_resolve = @required_resolve,
    visible_customer = @visible_customer, editable_customer = @editable_customer, show_when = sqlc.narg(show_when),
    ai_hint = sqlc.narg(ai_hint), ai_auto = @ai_auto, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: ArchiveTicketField :exec
-- ArchiveTicketField hides a field from forms; stored values stay.
UPDATE ticket_fields SET archived_at = @now, updated_at = @now WHERE workspace_id = @workspace_id AND id = @id;

-- name: SaveTicketFieldOrder :exec
-- SaveTicketFieldOrder stores a type's fields in the order given.
UPDATE ticket_fields f SET position = (o.ord - 1)::integer, updated_at = @now
FROM unnest(@ids::uuid[]) WITH ORDINALITY AS o(id, ord)
WHERE f.workspace_id = @workspace_id AND f.type_id = @type_id AND f.id = o.id;

-- name: SetInboxTicketType :exec
-- SetInboxTicketType makes every new conversation of the inbox a ticket
-- of the type, or stops it with null.
UPDATE inboxes SET ticket_type_id = sqlc.narg(ticket_type_id), updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id;

-- name: FindInboxTicketType :one
-- FindInboxTicketType returns the type new conversations of the inbox
-- become, null for none.
SELECT ticket_type_id FROM inboxes WHERE workspace_id = @workspace_id AND id = @id;

-- name: FindWorkspaceTicketSettings :one
-- FindWorkspaceTicketSettings returns the workspace's ticket settings
-- and spike threshold.
SELECT ticket_settings, incident_spike_threshold FROM workspaces WHERE id = @workspace_id;

-- name: SaveWorkspaceTicketSettings :exec
-- SaveWorkspaceTicketSettings stores the workspace's ticket settings and
-- spike threshold.
UPDATE workspaces SET ticket_settings = @ticket_settings, incident_spike_threshold = @incident_spike_threshold
WHERE id = @workspace_id;
