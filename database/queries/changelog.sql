-- name: ListChangelogLabels :many
-- ListChangelogLabels returns a help center's changelog labels in order.
SELECT * FROM changelog_labels
WHERE workspace_id = @workspace_id AND help_center_id = @help_center_id
ORDER BY position, id;

-- name: FindChangelogLabel :one
-- FindChangelogLabel returns one changelog label.
SELECT * FROM changelog_labels WHERE workspace_id = @workspace_id AND id = @id;

-- name: CountChangelogLabels :one
-- CountChangelogLabels counts a help center's changelog labels.
SELECT count(*) FROM changelog_labels WHERE workspace_id = @workspace_id AND help_center_id = @help_center_id;

-- name: ChangelogLabelSlugTaken :one
-- ChangelogLabelSlugTaken reports whether another label of the help
-- center has the slug.
SELECT EXISTS (
    SELECT 1 FROM changelog_labels
    WHERE workspace_id = @workspace_id AND help_center_id = @help_center_id AND slug = @slug AND id <> @id
) AS taken;

-- name: CreateChangelogLabel :one
-- CreateChangelogLabel adds a label at the end of the help center's list.
INSERT INTO changelog_labels (workspace_id, help_center_id, name, slug, color, position, created_at, updated_at)
SELECT @workspace_id, @help_center_id, @name, @slug, @color,
    (coalesce(max(position), 0) + 1024)::integer, @now, @now
FROM changelog_labels WHERE workspace_id = @workspace_id AND help_center_id = @help_center_id
RETURNING *;

-- name: UpdateChangelogLabel :one
-- UpdateChangelogLabel renames, recolors or moves a label.
UPDATE changelog_labels SET name = @name, slug = @slug, color = @color, position = @position, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: DeleteChangelogLabel :exec
-- DeleteChangelogLabel removes a label from every entry and deletes it.
DELETE FROM changelog_labels WHERE workspace_id = @workspace_id AND id = @id;

-- name: ListChangelogEntries :many
-- ListChangelogEntries returns a page of a help center's live entries,
-- narrowed by state, label and title when given, newest edit first.
SELECT * FROM changelog_entries e
WHERE e.workspace_id = @workspace_id AND e.help_center_id = @help_center_id AND e.deleted_at IS NULL
  AND (sqlc.narg(status)::text IS NULL OR e.status = sqlc.narg(status)::text)
  AND (sqlc.narg(label_id)::uuid IS NULL OR EXISTS (
      SELECT 1 FROM changelog_entry_labels l WHERE l.entry_id = e.id AND l.label_id = sqlc.narg(label_id)::uuid))
  AND (sqlc.narg(query)::text IS NULL OR e.title ILIKE '%' || sqlc.narg(query)::text || '%')
ORDER BY e.updated_at DESC, e.id DESC
LIMIT @row_limit OFFSET @row_offset;

-- name: CountChangelogEntries :one
-- CountChangelogEntries counts what ListChangelogEntries would list, up
-- to the cap.
SELECT count(*) FROM (
    SELECT 1 FROM changelog_entries e
    WHERE e.workspace_id = @workspace_id AND e.help_center_id = @help_center_id AND e.deleted_at IS NULL
      AND (sqlc.narg(status)::text IS NULL OR e.status = sqlc.narg(status)::text)
      AND (sqlc.narg(label_id)::uuid IS NULL OR EXISTS (
          SELECT 1 FROM changelog_entry_labels l WHERE l.entry_id = e.id AND l.label_id = sqlc.narg(label_id)::uuid))
      AND (sqlc.narg(query)::text IS NULL OR e.title ILIKE '%' || sqlc.narg(query)::text || '%')
    LIMIT @cap
) counted;

-- name: CountChangelogEntriesByStatus :many
-- CountChangelogEntriesByStatus counts a help center's live entries in
-- each state.
SELECT status, count(*)::integer AS entries FROM changelog_entries
WHERE workspace_id = @workspace_id AND help_center_id = @help_center_id AND deleted_at IS NULL
GROUP BY status;

-- name: FindChangelogEntry :one
-- FindChangelogEntry returns one live entry.
SELECT * FROM changelog_entries WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL;

-- name: LockChangelogEntry :one
-- LockChangelogEntry returns the entry and holds it until the
-- transaction ends.
SELECT * FROM changelog_entries WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL FOR UPDATE;

-- name: ChangelogSlugTaken :one
-- ChangelogSlugTaken reports whether another live entry of the help
-- center has the slug.
SELECT EXISTS (
    SELECT 1 FROM changelog_entries
    WHERE workspace_id = @workspace_id AND help_center_id = @help_center_id AND slug = @slug
      AND deleted_at IS NULL AND id <> @id
) AS taken;

-- name: CreateChangelogEntry :one
-- CreateChangelogEntry stores a new draft.
INSERT INTO changelog_entries (
    workspace_id, help_center_id, title, slug, summary, body_markdown, cover_url, created_at, updated_at
) VALUES (
    @workspace_id, @help_center_id, @title, @slug, @summary, @body_markdown, @cover_url, @now, @now
)
RETURNING *;

-- name: UpdateChangelogEntry :one
-- UpdateChangelogEntry writes the working copy, if nobody changed it
-- since the lock version the editor read.
UPDATE changelog_entries SET
    title = @title, slug = @slug, summary = @summary, body_markdown = @body_markdown, cover_url = @cover_url,
    has_unpublished_changes = (status = 'published'), authors_chosen = @authors_chosen,
    lock_version = lock_version + 1, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL AND lock_version = @lock_version
RETURNING *;

-- name: SetChangelogPublishDate :one
-- SetChangelogPublishDate moves when a published entry says it shipped.
UPDATE changelog_entries SET published_at = @published_at, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL AND status = 'published'
RETURNING *;

-- name: PublishChangelogEntry :one
-- PublishChangelogEntry copies the working copy over what readers see.
-- A first publish dates the entry now unless a date is given; later
-- publishes keep its date.
UPDATE changelog_entries SET
    status = 'published', scheduled_for = NULL,
    published_at = coalesce(sqlc.narg(published_at)::timestamptz, published_at, @now),
    published_title = title, published_summary = summary, published_body_markdown = body_markdown,
    published_body_html = @body_html, published_renderer_version = @renderer_version, published_cover_url = cover_url,
    has_unpublished_changes = false, notify_subscribers = notify_subscribers OR @notify,
    lock_version = lock_version + 1, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL
RETURNING *;

-- name: ScheduleChangelogEntry :one
-- ScheduleChangelogEntry holds a draft to publish itself later.
UPDATE changelog_entries SET
    status = 'scheduled', scheduled_for = @scheduled_for, notify_subscribers = @notify,
    lock_version = lock_version + 1, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL AND status <> 'published'
RETURNING *;

-- name: UnpublishChangelogEntry :one
-- UnpublishChangelogEntry takes an entry back to a draft readers no
-- longer see, scheduled or published.
UPDATE changelog_entries SET
    status = 'draft', scheduled_for = NULL, has_unpublished_changes = false,
    lock_version = lock_version + 1, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL
RETURNING *;

-- name: ClaimChangelogMailing :one
-- ClaimChangelogMailing marks the entry's one mailing as sent, if it
-- asked for one and none went out yet.
UPDATE changelog_entries SET notified_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL AND status = 'published'
  AND notify_subscribers AND notified_at IS NULL
RETURNING *;

-- name: DeleteChangelogEntry :exec
-- DeleteChangelogEntry deletes an entry; its slug is free again.
UPDATE changelog_entries SET deleted_at = @now, status = 'draft', updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL;

-- name: DueChangelogEntries :many
-- DueChangelogEntries lists the workspace's scheduled entries whose time
-- has come.
SELECT id FROM changelog_entries
WHERE workspace_id = @workspace_id AND status = 'scheduled' AND deleted_at IS NULL AND scheduled_for <= @now
ORDER BY scheduled_for
LIMIT 100;

-- name: ListChangelogEntryLabels :many
-- ListChangelogEntryLabels returns the labels of the entries, in label
-- order.
SELECT el.entry_id, l.id, l.name, l.slug, l.color
FROM changelog_entry_labels el
JOIN changelog_labels l ON l.id = el.label_id
WHERE el.workspace_id = @workspace_id AND el.entry_id = ANY(@entry_ids::uuid[])
ORDER BY l.position, l.id;

-- name: ClearChangelogEntryLabels :exec
-- ClearChangelogEntryLabels takes every label off an entry.
DELETE FROM changelog_entry_labels WHERE workspace_id = @workspace_id AND entry_id = @entry_id;

-- name: AddChangelogEntryLabels :exec
-- AddChangelogEntryLabels files an entry under the labels of its help
-- center among those given.
INSERT INTO changelog_entry_labels (entry_id, label_id, workspace_id)
SELECT @entry_id, l.id, @workspace_id FROM changelog_labels l
WHERE l.workspace_id = @workspace_id AND l.help_center_id = @help_center_id AND l.id = ANY(@label_ids::uuid[])
ON CONFLICT DO NOTHING;

-- name: ListChangelogEntryPeople :many
-- ListChangelogEntryPeople returns the editors and chosen authors of the
-- entries, with their names and pictures.
SELECT p.entry_id, p.role, p.position, p.member_id, u.name, u.avatar_path, p.created_at
FROM changelog_entry_people p
JOIN workspace_members m ON m.id = p.member_id
JOIN users u ON u.id = m.user_id
WHERE p.workspace_id = @workspace_id AND p.entry_id = ANY(@entry_ids::uuid[])
ORDER BY p.role, p.position, p.created_at, p.member_id;

-- name: AddChangelogEditor :exec
-- AddChangelogEditor records that a teammate edited the entry, once.
INSERT INTO changelog_entry_people (entry_id, member_id, workspace_id, role, position, created_at)
VALUES (@entry_id, @member_id, @workspace_id, 'editor', 0, @now)
ON CONFLICT DO NOTHING;

-- name: ClearChangelogAuthors :exec
-- ClearChangelogAuthors forgets the authors chosen for an entry.
DELETE FROM changelog_entry_people WHERE workspace_id = @workspace_id AND entry_id = @entry_id AND role = 'author';

-- name: AddChangelogAuthor :exec
-- AddChangelogAuthor names a teammate of the workspace as an author.
INSERT INTO changelog_entry_people (entry_id, member_id, workspace_id, role, position, created_at)
SELECT @entry_id, m.id, @workspace_id, 'author', @position, @now FROM workspace_members m
WHERE m.workspace_id = @workspace_id AND m.id = @member_id
ON CONFLICT DO NOTHING;

-- name: ListPublishedChangelogEntries :many
-- ListPublishedChangelogEntries returns a page of what readers see of a
-- changelog, newest first, narrowed to a label when given.
SELECT * FROM changelog_entries e
WHERE e.workspace_id = @workspace_id AND e.help_center_id = @help_center_id AND e.deleted_at IS NULL
  AND e.status = 'published'
  AND (sqlc.narg(label_id)::uuid IS NULL OR EXISTS (
      SELECT 1 FROM changelog_entry_labels l WHERE l.entry_id = e.id AND l.label_id = sqlc.narg(label_id)::uuid))
ORDER BY e.published_at DESC, e.id DESC
LIMIT @row_limit OFFSET @row_offset;

-- name: CountPublishedChangelogEntries :one
-- CountPublishedChangelogEntries counts what readers see of a changelog,
-- narrowed to a label when given.
SELECT count(*) FROM changelog_entries e
WHERE e.workspace_id = @workspace_id AND e.help_center_id = @help_center_id AND e.deleted_at IS NULL
  AND e.status = 'published'
  AND (sqlc.narg(label_id)::uuid IS NULL OR EXISTS (
      SELECT 1 FROM changelog_entry_labels l WHERE l.entry_id = e.id AND l.label_id = sqlc.narg(label_id)::uuid));

-- name: CountPublishedChangelogLabels :many
-- CountPublishedChangelogLabels counts readers' entries under each label.
SELECT el.label_id, count(*)::integer AS entries
FROM changelog_entry_labels el
JOIN changelog_entries e ON e.id = el.entry_id
WHERE el.workspace_id = @workspace_id AND e.help_center_id = @help_center_id AND e.deleted_at IS NULL
  AND e.status = 'published'
GROUP BY el.label_id;

-- name: ListChangelogPublishDates :many
-- ListChangelogPublishDates returns when each published entry of the
-- year shipped, newest first, for the release strip.
SELECT published_at::timestamptz AS published_at, slug, published_title FROM changelog_entries
WHERE workspace_id = @workspace_id AND help_center_id = @help_center_id AND deleted_at IS NULL
  AND status = 'published' AND published_at >= @since AND published_at < @until
ORDER BY published_at DESC;

-- name: FindPublishedChangelogEntryBySlug :one
-- FindPublishedChangelogEntryBySlug returns a published entry by slug.
SELECT * FROM changelog_entries
WHERE workspace_id = @workspace_id AND help_center_id = @help_center_id AND slug = @slug
  AND deleted_at IS NULL AND status = 'published';

-- name: FindPublishedChangelogNeighbors :many
-- FindPublishedChangelogNeighbors returns the published entries just
-- newer and just older than the given one.
(SELECT id, slug, published_title, published_at, 'newer'::text AS side FROM changelog_entries n
 WHERE n.workspace_id = @workspace_id AND n.help_center_id = @help_center_id AND n.deleted_at IS NULL
   AND n.status = 'published' AND (n.published_at, n.id) > (@published_at::timestamptz, @id::uuid)
 ORDER BY n.published_at, n.id LIMIT 1)
UNION ALL
(SELECT id, slug, published_title, published_at, 'older'::text AS side FROM changelog_entries o
 WHERE o.workspace_id = @workspace_id AND o.help_center_id = @help_center_id AND o.deleted_at IS NULL
   AND o.status = 'published' AND (o.published_at, o.id) < (@published_at::timestamptz, @id::uuid)
 ORDER BY o.published_at DESC, o.id DESC LIMIT 1);

-- name: CountChangelogReactions :many
-- CountChangelogReactions counts each emoji on the entries.
SELECT entry_id, emoji, count(*)::integer AS reactions FROM changelog_reactions
WHERE workspace_id = @workspace_id AND entry_id = ANY(@entry_ids::uuid[])
GROUP BY entry_id, emoji;

-- name: AddChangelogReaction :exec
-- AddChangelogReaction counts one reader's emoji on an entry once.
INSERT INTO changelog_reactions (entry_id, workspace_id, reactor_hash, emoji, created_at)
VALUES (@entry_id, @workspace_id, @reactor_hash, @emoji, @now)
ON CONFLICT DO NOTHING;

-- name: FindChangelogSubscriberByEmail :one
-- FindChangelogSubscriberByEmail returns the help center's subscriber
-- with the address.
SELECT * FROM changelog_subscribers
WHERE workspace_id = @workspace_id AND help_center_id = @help_center_id AND email = @email;

-- name: FindChangelogSubscriber :one
-- FindChangelogSubscriber returns one subscriber.
SELECT * FROM changelog_subscribers WHERE workspace_id = @workspace_id AND id = @id;

-- name: UpsertChangelogSubscriber :one
-- UpsertChangelogSubscriber adds an address, or brings back one that left
-- or never confirmed, in the given state and with a new nonce. An active
-- subscriber stays active.
INSERT INTO changelog_subscribers (
    workspace_id, help_center_id, email, contact_id, status, nonce, confirmation_sent_at, confirmed_at,
    created_at, updated_at
) VALUES (
    @workspace_id, @help_center_id, @email, @contact_id, @status, @nonce, @sent_at, @confirmed_at, @now, @now
)
ON CONFLICT (help_center_id, email) DO UPDATE SET
    status = CASE WHEN changelog_subscribers.status = 'active' THEN 'active' ELSE excluded.status END,
    nonce = CASE WHEN changelog_subscribers.status = 'active' THEN changelog_subscribers.nonce ELSE excluded.nonce END,
    contact_id = coalesce(excluded.contact_id, changelog_subscribers.contact_id),
    confirmation_sent_at = coalesce(excluded.confirmation_sent_at, changelog_subscribers.confirmation_sent_at),
    confirmed_at = coalesce(changelog_subscribers.confirmed_at, excluded.confirmed_at),
    unsubscribed_at = CASE WHEN excluded.status = 'active' THEN NULL ELSE changelog_subscribers.unsubscribed_at END,
    updated_at = excluded.updated_at
RETURNING *;

-- name: ConfirmChangelogSubscriber :one
-- ConfirmChangelogSubscriber makes a pending or departed subscriber
-- active.
UPDATE changelog_subscribers SET status = 'active', confirmed_at = coalesce(confirmed_at, @now),
    unsubscribed_at = NULL, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: UnsubscribeChangelogSubscriber :one
-- UnsubscribeChangelogSubscriber stops mail to a subscriber and kills
-- their old links.
UPDATE changelog_subscribers SET status = 'unsubscribed', unsubscribed_at = @now, nonce = @nonce, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: DeleteChangelogSubscriber :exec
-- DeleteChangelogSubscriber forgets a subscriber.
DELETE FROM changelog_subscribers WHERE workspace_id = @workspace_id AND id = @id;

-- name: ListChangelogSubscribers :many
-- ListChangelogSubscribers returns a page of a help center's subscribers,
-- narrowed by state and address when given, newest first.
SELECT * FROM changelog_subscribers
WHERE workspace_id = @workspace_id AND help_center_id = @help_center_id
  AND (sqlc.narg(status)::text IS NULL OR status = sqlc.narg(status)::text)
  AND (sqlc.narg(query)::text IS NULL OR email ILIKE '%' || sqlc.narg(query)::text || '%')
ORDER BY created_at DESC, id DESC
LIMIT @row_limit OFFSET @row_offset;

-- name: CountChangelogSubscribers :one
-- CountChangelogSubscribers counts what ListChangelogSubscribers would
-- list.
SELECT count(*) FROM changelog_subscribers
WHERE workspace_id = @workspace_id AND help_center_id = @help_center_id
  AND (sqlc.narg(status)::text IS NULL OR status = sqlc.narg(status)::text)
  AND (sqlc.narg(query)::text IS NULL OR email ILIKE '%' || sqlc.narg(query)::text || '%');

-- name: ListActiveChangelogSubscribers :many
-- ListActiveChangelogSubscribers returns the next batch of active
-- subscribers after the given id, for a mailing.
SELECT * FROM changelog_subscribers
WHERE workspace_id = @workspace_id AND help_center_id = @help_center_id AND status = 'active' AND id > @after
ORDER BY id
LIMIT @row_limit;
