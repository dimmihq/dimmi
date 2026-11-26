-- name: CreateMigration :one
-- CreateMigration starts a migration from an account of another tool.
INSERT INTO migrations (workspace_id, source, source_account, name, created_by_member_id, created_at, updated_at)
VALUES (@workspace_id, @source, @source_account, sqlc.narg(name), sqlc.narg(created_by_member_id), @now, @now)
RETURNING *;

-- name: FindMigration :one
SELECT * FROM migrations WHERE workspace_id = @workspace_id AND id = @id;

-- name: LockMigration :one
-- LockMigration holds the migration until the transaction ends, so a
-- batch and an undo never run at once.
SELECT * FROM migrations WHERE workspace_id = @workspace_id AND id = @id FOR UPDATE;

-- name: FindLiveMigration :one
-- FindLiveMigration returns the migration of the account that was not
-- undone.
SELECT * FROM migrations
WHERE workspace_id = @workspace_id AND source = @source AND source_account = @source_account
  AND status <> 'rolled_back';

-- name: ListMigrations :many
-- ListMigrations returns the workspace's migrations, newest first.
SELECT * FROM migrations WHERE workspace_id = @workspace_id ORDER BY created_at DESC, id DESC LIMIT @row_limit;

-- name: RenameMigration :one
UPDATE migrations SET name = sqlc.narg(name), updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: TouchMigration :exec
-- TouchMigration notes when the migration last received records.
UPDATE migrations SET last_record_at = @now::timestamptz, updated_at = @now::timestamptz WHERE workspace_id = @workspace_id AND id = @id;

-- name: RequestMigrationRollback :one
-- RequestMigrationRollback marks the migration to be undone, once.
UPDATE migrations
SET status = 'rolling_back', rollback_by_member_id = @member_id::uuid, rollback_requested_at = @now::timestamptz,
    rollback_error = NULL, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id AND status IN ('active', 'rollback_failed')
RETURNING *;

-- name: FinishMigrationRollback :exec
-- FinishMigrationRollback ends an undo, rolled back or failed with why.
UPDATE migrations
SET status = @status::text, rollback_removed = rollback_removed + @removed, rollback_error = sqlc.narg(error),
    rollback_finished_at = @now::timestamptz, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id;

-- name: FindMigrationRecord :one
SELECT * FROM migration_records
WHERE migration_id = @migration_id AND kind = @kind AND source_id = @source_id;

-- name: FindMigrationRecords :many
-- FindMigrationRecords returns the records of the kind with the source
-- ids, leaving out those an undo removed.
SELECT * FROM migration_records
WHERE migration_id = @migration_id AND kind = @kind AND source_id = ANY (@source_ids::text[]) AND undone_at IS NULL;

-- name: SaveMigrationRecord :one
-- SaveMigrationRecord keeps what a record became. A record that was
-- created once stays created, so an undo still removes it.
INSERT INTO migration_records (
    workspace_id, migration_id, kind, source_id, status, record_id, created, label, content_hash,
    source_updated_at, payload, waiting_for, state, error, warnings, conflicts, created_at, updated_at
) VALUES (
    @workspace_id, @migration_id, @kind, @source_id, @status, sqlc.narg(record_id), @created, sqlc.narg(label),
    @content_hash, sqlc.narg(source_updated_at), sqlc.narg(payload), @waiting_for, @state, sqlc.narg(error),
    @warnings, @conflicts, @now, @now
)
ON CONFLICT (migration_id, kind, source_id) DO UPDATE
SET status = EXCLUDED.status, record_id = coalesce(EXCLUDED.record_id, migration_records.record_id),
    created = migration_records.created OR EXCLUDED.created, label = coalesce(EXCLUDED.label, migration_records.label),
    content_hash = EXCLUDED.content_hash, source_updated_at = EXCLUDED.source_updated_at,
    payload = EXCLUDED.payload, waiting_for = EXCLUDED.waiting_for, state = EXCLUDED.state, error = EXCLUDED.error,
    warnings = EXCLUDED.warnings, conflicts = EXCLUDED.conflicts, updated_at = EXCLUDED.updated_at
RETURNING *;

-- name: ListDeferredMigrationRecords :many
-- ListDeferredMigrationRecords returns the records still waiting for a
-- reference, oldest first.
SELECT * FROM migration_records
WHERE migration_id = @migration_id AND status = 'deferred'
ORDER BY updated_at, id
LIMIT @row_limit;

-- name: CountMigrationRecords :many
-- CountMigrationRecords counts the migration's records by kind and
-- status.
SELECT kind, status, count(*)::integer AS records, count(*) FILTER (WHERE created)::integer AS created,
    count(*) FILTER (WHERE jsonb_array_length(conflicts) > 0)::integer AS conflicted
FROM migration_records
WHERE migration_id = @migration_id AND undone_at IS NULL
GROUP BY kind, status
ORDER BY kind, status;

-- name: ListMigrationProblems :many
-- ListMigrationProblems returns the latest records that failed, are
-- unsupported or carry conflicts.
SELECT * FROM migration_records
WHERE migration_id = @migration_id
  AND (status IN ('failed', 'unsupported') OR jsonb_array_length(conflicts) > 0)
ORDER BY updated_at DESC, id DESC
LIMIT @row_limit;

-- name: ListCreatedMigrationRecords :many
-- ListCreatedMigrationRecords returns a page of the records of the kind
-- the migration created, newest first, for an undo.
SELECT * FROM migration_records
WHERE migration_id = @migration_id AND kind = @kind AND created AND record_id IS NOT NULL AND undone_at IS NULL
ORDER BY id DESC
LIMIT @row_limit;

-- name: UndoMigrationRecord :exec
-- UndoMigrationRecord marks a record the undo removed. The row stays, so
-- a later migration of the account can bring back what the undo trashed.
UPDATE migration_records SET undone_at = @now::timestamptz, updated_at = @now::timestamptz WHERE migration_id = @migration_id AND id = @id;

-- name: ReviveUndoneContact :one
-- ReviveUndoneContact restores the trashed contact with the external id
-- when an undone migration created it, so running again finds it.
UPDATE contacts c SET deleted_at = NULL, updated_at = @now::timestamptz
WHERE c.workspace_id = @workspace_id AND c.external_id = @external_id::text AND c.deleted_at IS NOT NULL
  AND EXISTS (
    SELECT 1 FROM migration_records r
    WHERE r.workspace_id = c.workspace_id AND r.record_id = c.id AND r.kind = 'contact' AND r.created
      AND r.undone_at IS NOT NULL
  )
RETURNING c.id;

-- name: ReviveUndoneCompany :one
-- ReviveUndoneCompany restores the trashed company with the external id
-- when an undone migration created it, so running again finds it.
UPDATE companies c SET deleted_at = NULL, updated_at = @now::timestamptz
WHERE c.workspace_id = @workspace_id AND c.external_id = @external_id::text AND c.deleted_at IS NOT NULL
  AND EXISTS (
    SELECT 1 FROM migration_records r
    WHERE r.workspace_id = c.workspace_id AND r.record_id = c.id AND r.kind = 'company' AND r.created
      AND r.undone_at IS NOT NULL
  )
RETURNING c.id;

-- name: MigrationFileByAttachment :one
-- MigrationFileByAttachment returns the migration file record of the
-- attachment.
SELECT * FROM migration_records
WHERE workspace_id = @workspace_id AND kind = 'file' AND record_id = @record_id
LIMIT 1;

-- name: FindLiveTeamByName :one
-- FindLiveTeamByName returns the workspace's live team with the name,
-- whatever its case.
SELECT * FROM teams WHERE workspace_id = @workspace_id AND lower(name) = lower(@name::text) AND deleted_at IS NULL;

-- name: MigrateContact :one
-- MigrateContact writes what the other tool knew about a contact: each
-- detail it sent, keeping the one here when it sent none, and the times
-- the contact was first and last seen.
UPDATE contacts
SET kind = coalesce(sqlc.narg(kind), kind), external_id = coalesce(sqlc.narg(external_id), external_id),
    email = coalesce(sqlc.narg(email)::citext, email), name = coalesce(sqlc.narg(name), name),
    phone = coalesce(sqlc.narg(phone), phone), avatar_url = coalesce(sqlc.narg(avatar_url), avatar_url),
    locale = coalesce(sqlc.narg(locale), locale), timezone = coalesce(sqlc.narg(timezone), timezone),
    location = coalesce(sqlc.narg(location), location), company_id = coalesce(sqlc.narg(company_id), company_id),
    custom_attributes = custom_attributes || @custom_attributes::jsonb,
    first_seen_at = coalesce(sqlc.narg(first_seen_at), first_seen_at),
    last_seen_at = coalesce(sqlc.narg(last_seen_at), last_seen_at),
    created_at = least(created_at, coalesce(sqlc.narg(created_at), created_at)), updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: MigrateCompanyTimes :exec
-- MigrateCompanyTimes backdates a company to when it was made in the
-- other tool.
UPDATE companies SET created_at = least(created_at, @created_at::timestamptz)
WHERE workspace_id = @workspace_id AND id = @id;

-- name: CreateMigratedNote :one
-- CreateMigratedNote adds a note about a customer or company at the time
-- it was written.
INSERT INTO customer_notes (workspace_id, contact_id, company_id, author_member_id, body, created_at, updated_at)
VALUES (@workspace_id, sqlc.narg(contact_id), sqlc.narg(company_id), sqlc.narg(author_member_id), @body,
    @created_at, @now)
RETURNING id;

-- name: UpdateMigratedNote :execrows
-- UpdateMigratedNote rewrites an imported note's body.
UPDATE customer_notes SET body = @body, updated_at = @now WHERE workspace_id = @workspace_id AND id = @id;

-- name: FindTagByName :one
-- FindTagByName returns the workspace's tag with the name, whatever its
-- case.
SELECT * FROM tags WHERE workspace_id = @workspace_id AND name = @name::citext;

-- name: CountHelpCenterArticles :one
-- CountHelpCenterArticles counts the live articles of a help center.
SELECT count(*)::integer FROM articles
WHERE workspace_id = @workspace_id AND help_center_id = @help_center_id AND deleted_at IS NULL;

-- name: MigrateArticle :exec
-- MigrateArticle gives an imported article its author and the times it
-- was written and first published in the other tool.
UPDATE articles
SET author_member_id = coalesce(sqlc.narg(author_member_id), author_member_id),
    created_at = coalesce(sqlc.narg(created_at), created_at),
    first_published_at = CASE WHEN first_published_at IS NULL THEN NULL
        ELSE coalesce(sqlc.narg(published_at), first_published_at) END,
    published_at = CASE WHEN published_at IS NULL THEN NULL ELSE coalesce(sqlc.narg(updated_at), published_at) END
WHERE workspace_id = @workspace_id AND id = @id;

-- name: MigrateCollection :one
-- MigrateCollection renames an imported collection and moves it.
UPDATE collections SET name = @name, description = sqlc.narg(description), parent_id = sqlc.narg(parent_id),
    updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL
RETURNING *;

-- name: FindInboxConnectionOfType :one
-- FindInboxConnectionOfType returns the inbox's oldest live connection of
-- the channel type.
SELECT * FROM channel_connections
WHERE workspace_id = @workspace_id AND inbox_id = @inbox_id AND type = @type AND deleted_at IS NULL
ORDER BY created_at, id
LIMIT 1;

-- name: MigrateRating :exec
-- MigrateRating keeps how the customer rated a closed conversation in
-- the other tool, once per close.
INSERT INTO conversation_ratings (
    workspace_id, conversation_id, contact_id, member_id, channel, closed_at, ask_at, asked_at, score, comment,
    rated_at, created_at, updated_at
) VALUES (
    @workspace_id, @conversation_id, @contact_id, sqlc.narg(member_id), @channel, @closed_at, @rated_at, @rated_at,
    @score, sqlc.narg(comment), @rated_at, @rated_at, @rated_at
)
ON CONFLICT (conversation_id, closed_at) DO UPDATE
SET score = EXCLUDED.score, comment = EXCLUDED.comment, rated_at = EXCLUDED.rated_at, updated_at = EXCLUDED.updated_at;

-- name: RemoveMigratedRecord :execrows
-- RemoveMigratedRecord undoes one record a migration created, by its
-- ledger kind: trashing what can be trashed, archiving fields and ticket
-- setup, and deleting the rest. A trashed ticket gives up its external
-- id, which is the migration's own, so running again makes it anew. Records others now use are kept: a
-- contact with conversations the migration did not bring, a tag, team or
-- company still in use, a help center with other articles, a schedule
-- an inbox follows.
WITH target AS (SELECT @kind::text AS kind, @id::uuid AS id, @workspace_id::uuid AS ws, @now::timestamptz AS now),
conv AS (
    UPDATE conversations c SET deleted_at = t.now, updated_at = t.now FROM target t
    WHERE t.kind = 'conversation' AND c.workspace_id = t.ws AND c.id = t.id AND c.deleted_at IS NULL
    RETURNING c.id
),
tick AS (
    UPDATE tickets k SET deleted_at = t.now, external_id = NULL, updated_at = t.now FROM target t
    WHERE t.kind = 'ticket' AND k.workspace_id = t.ws AND k.id = t.id AND k.deleted_at IS NULL
    RETURNING k.id
),
art AS (
    UPDATE articles a SET deleted_at = t.now, updated_at = t.now FROM target t
    WHERE t.kind IN ('article', 'snippet') AND a.workspace_id = t.ws AND a.id = t.id AND a.deleted_at IS NULL
    RETURNING a.id
),
col AS (
    UPDATE collections c SET deleted_at = t.now, updated_at = t.now FROM target t
    WHERE t.kind = 'collection' AND c.workspace_id = t.ws AND c.id = t.id AND c.deleted_at IS NULL
      AND NOT EXISTS (SELECT 1 FROM articles a WHERE a.collection_id = c.id AND a.deleted_at IS NULL)
      AND NOT EXISTS (SELECT 1 FROM collections k WHERE k.parent_id = c.id AND k.deleted_at IS NULL)
    RETURNING c.id
),
hc AS (
    UPDATE help_centers h SET deleted_at = t.now, updated_at = t.now FROM target t
    WHERE t.kind = 'help_center' AND h.workspace_id = t.ws AND h.id = t.id AND h.deleted_at IS NULL
      AND NOT h.is_default
      AND NOT EXISTS (SELECT 1 FROM articles a WHERE a.help_center_id = h.id AND a.deleted_at IS NULL)
    RETURNING h.id
),
note AS (
    DELETE FROM customer_notes n USING target t
    WHERE t.kind = 'contact_note' AND n.workspace_id = t.ws AND n.id = t.id
    RETURNING n.id
),
reply AS (
    DELETE FROM saved_replies r USING target t
    WHERE t.kind = 'saved_reply' AND r.workspace_id = t.ws AND r.id = t.id
    RETURNING r.id
),
sched AS (
    UPDATE schedules s SET deleted_at = t.now, updated_at = t.now FROM target t
    WHERE t.kind = 'schedule' AND s.workspace_id = t.ws AND s.id = t.id AND s.deleted_at IS NULL AND NOT s.is_default
      AND NOT EXISTS (SELECT 1 FROM inboxes i WHERE i.schedule_id = s.id)
    RETURNING s.id
),
contact AS (
    UPDATE contacts c SET deleted_at = t.now, updated_at = t.now FROM target t
    WHERE t.kind = 'contact' AND c.workspace_id = t.ws AND c.id = t.id AND c.deleted_at IS NULL
      AND NOT EXISTS (SELECT 1 FROM conversations v WHERE v.contact_id = c.id AND v.deleted_at IS NULL)
    RETURNING c.id
),
company AS (
    UPDATE companies c SET deleted_at = t.now, updated_at = t.now FROM target t
    WHERE t.kind = 'company' AND c.workspace_id = t.ws AND c.id = t.id AND c.deleted_at IS NULL
      AND NOT EXISTS (SELECT 1 FROM contacts k WHERE k.company_id = c.id AND k.deleted_at IS NULL)
    RETURNING c.id
),
team AS (
    UPDATE teams m SET deleted_at = t.now, updated_at = t.now FROM target t
    WHERE t.kind = 'team' AND m.workspace_id = t.ws AND m.id = t.id AND m.deleted_at IS NULL
      AND NOT EXISTS (SELECT 1 FROM conversations v WHERE v.assignee_team_id = m.id AND v.deleted_at IS NULL)
    RETURNING m.id
),
tag AS (
    DELETE FROM tags g USING target t
    WHERE t.kind = 'tag' AND g.workspace_id = t.ws AND g.id = t.id
      AND NOT EXISTS (
        SELECT 1 FROM conversation_tag x JOIN conversations v ON v.id = x.conversation_id
        WHERE x.tag_id = g.id AND v.deleted_at IS NULL
      )
      AND NOT EXISTS (
        SELECT 1 FROM contact_tag x JOIN contacts k ON k.id = x.contact_id WHERE x.tag_id = g.id AND k.deleted_at IS NULL
      )
      AND NOT EXISTS (
        SELECT 1 FROM company_tag x JOIN companies k ON k.id = x.company_id WHERE x.tag_id = g.id AND k.deleted_at IS NULL
      )
    RETURNING g.id
),
attr AS (
    UPDATE attribute_definitions d SET status = 'archived', updated_at = t.now FROM target t
    WHERE t.kind = 'attribute_definition' AND d.workspace_id = t.ws AND d.id = t.id AND d.status = 'active'
    RETURNING d.id
),
ttype AS (
    UPDATE ticket_types y SET archived_at = t.now, updated_at = t.now FROM target t
    WHERE t.kind = 'ticket_type' AND y.workspace_id = t.ws AND y.id = t.id AND y.archived_at IS NULL
      AND NOT EXISTS (SELECT 1 FROM tickets k WHERE k.type_id = y.id AND k.deleted_at IS NULL)
    RETURNING y.id
),
file AS (
    DELETE FROM attachments a USING target t
    WHERE t.kind = 'file' AND a.workspace_id = t.ws AND a.id = t.id AND a.message_id IS NULL
    RETURNING a.id
)
SELECT id FROM conv UNION ALL SELECT id FROM tick UNION ALL SELECT id FROM art UNION ALL SELECT id FROM col
UNION ALL SELECT id FROM hc UNION ALL SELECT id FROM note UNION ALL SELECT id FROM reply
UNION ALL SELECT id FROM sched UNION ALL SELECT id FROM contact UNION ALL SELECT id FROM company
UNION ALL SELECT id FROM team UNION ALL SELECT id FROM tag UNION ALL SELECT id FROM attr
UNION ALL SELECT id FROM ttype UNION ALL SELECT id FROM file;

-- name: ShareMigration :one
-- ShareMigration holds the migration against an undo until the
-- transaction ends, while other batches of it still run.
SELECT * FROM migrations WHERE workspace_id = @workspace_id AND id = @id FOR SHARE;
