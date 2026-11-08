-- name: ListEventDefinitions :many
-- ListEventDefinitions returns the workspace's event names.
SELECT * FROM event_definitions WHERE workspace_id = @workspace_id ORDER BY name;

-- name: FindEventDefinition :one
-- FindEventDefinition returns one event definition.
SELECT * FROM event_definitions WHERE workspace_id = @workspace_id AND id = @id;

-- name: FindEventDefinitionByName :one
-- FindEventDefinitionByName returns the definition of a name.
SELECT * FROM event_definitions WHERE workspace_id = @workspace_id AND name = @name;

-- name: CountActiveEventDefinitions :one
-- CountActiveEventDefinitions counts the active names, which a workspace
-- may have 1,000 of.
SELECT count(*) FROM event_definitions WHERE workspace_id = @workspace_id AND status = 'active';

-- name: CreateEventDefinition :one
-- CreateEventDefinition stores a definition, leaving one that already
-- exists for the name as it is.
INSERT INTO event_definitions (id, workspace_id, name, label, description, origin, created_at, updated_at)
VALUES (@id, @workspace_id, @name, @label, @description, @origin, @now, @now)
ON CONFLICT (workspace_id, name) DO NOTHING
RETURNING *;

-- name: UpdateEventDefinition :one
-- UpdateEventDefinition changes a definition's words and status.
UPDATE event_definitions SET label = @label, description = @description, status = @status, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: CountEventDefinition :exec
-- CountEventDefinition adds events to a name's total and its last time.
UPDATE event_definitions SET total_count = total_count + @added::bigint,
    last_seen_at = greatest(coalesce(last_seen_at, @seen::timestamptz), @seen::timestamptz)
WHERE workspace_id = @workspace_id AND id = @id;

-- name: LockEventSummary :one
-- LockEventSummary returns a person's summary of one event, locked for
-- the write that follows.
SELECT * FROM contact_event_summaries
WHERE workspace_id = @workspace_id AND contact_id = @contact_id AND event_definition_id = @event_definition_id
FOR UPDATE;

-- name: SaveEventSummary :exec
-- SaveEventSummary stores a person's summary of one event.
INSERT INTO contact_event_summaries (workspace_id, contact_id, event_definition_id, count, first_at, last_at,
    daily_day, daily, last_metadata, updated_at)
VALUES (@workspace_id, @contact_id, @event_definition_id, @count, @first_at, @last_at, @daily_day, @daily,
    @last_metadata, @now)
ON CONFLICT (workspace_id, contact_id, event_definition_id) DO UPDATE SET
    count = EXCLUDED.count, first_at = EXCLUDED.first_at, last_at = EXCLUDED.last_at,
    daily_day = EXCLUDED.daily_day, daily = EXCLUDED.daily, last_metadata = EXCLUDED.last_metadata,
    updated_at = EXCLUDED.updated_at;

-- name: ListContactEventSummaries :many
-- ListContactEventSummaries returns people's event summaries with the
-- event names, newest first.
SELECT s.*, d.name, d.label FROM contact_event_summaries s
JOIN event_definitions d ON d.id = s.event_definition_id AND d.workspace_id = s.workspace_id
WHERE s.workspace_id = @workspace_id AND s.contact_id = ANY (@contact_ids::uuid[])
ORDER BY s.contact_id, s.last_at DESC;

-- name: FindEventContactByEmail :one
-- FindEventContactByEmail returns the person an event names by email:
-- live and unmerged, users first, then a proven address, then the one
-- seen most recently.
SELECT * FROM contacts
WHERE workspace_id = @workspace_id AND email = @email AND merged_into_id IS NULL AND deleted_at IS NULL
ORDER BY (kind = 'user') DESC, email_verified DESC, last_seen_at DESC NULLS LAST, id DESC
LIMIT 1;

-- name: DeleteContactEventSummaries :exec
-- DeleteContactEventSummaries removes a person's summaries once they were
-- added to the person they turned out to be.
DELETE FROM contact_event_summaries WHERE workspace_id = @workspace_id AND contact_id = @contact_id;
