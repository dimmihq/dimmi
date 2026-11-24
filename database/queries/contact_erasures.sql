-- name: CreateContactErasure :one
INSERT INTO contact_erasures (
    workspace_id, contact_id, status, requested_by_member_id, scheduled_for, identities_stash, created_at, updated_at
) VALUES (
    @workspace_id, @contact_id, 'pending', @requested_by_member_id, @scheduled_for, @identities_stash, @now, @now
)
RETURNING *;

-- name: FindContactErasure :one
SELECT * FROM contact_erasures WHERE workspace_id = @workspace_id AND id = @id;

-- name: LockContactErasure :one
-- LockContactErasure returns the request and holds it for the transaction,
-- so a cancel and the job never act on it at once.
SELECT * FROM contact_erasures WHERE workspace_id = @workspace_id AND id = @id FOR UPDATE;

-- name: ListContactErasures :many
-- ListContactErasures returns a page of the workspace's requests, newest
-- first, narrowed to one status when one is given.
SELECT * FROM contact_erasures
WHERE workspace_id = @workspace_id
  AND (sqlc.narg(status)::text IS NULL OR status = sqlc.narg(status)::text)
ORDER BY created_at DESC, id DESC
LIMIT @row_limit::bigint OFFSET @row_offset::bigint;

-- name: CountContactErasures :one
-- CountContactErasures counts the requests ListContactErasures would
-- list, up to the cap.
SELECT count(*) FROM (
SELECT 1 FROM contact_erasures
WHERE workspace_id = @workspace_id
  AND (sqlc.narg(status)::text IS NULL OR status = sqlc.narg(status)::text)
LIMIT @row_limit::bigint
) AS counted;

-- name: SetContactErasureStatus :one
UPDATE contact_erasures SET status = @status, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: CancelContactErasure :one
-- CancelContactErasure ends the request and drops the stashed identities.
UPDATE contact_erasures SET status = 'canceled', identities_stash = NULL, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: CompleteContactErasure :one
-- CompleteContactErasure records what the erasure removed and drops the
-- stashed identities for good.
UPDATE contact_erasures
SET status = 'completed', completed_at = @now, counts = @counts, identities_stash = NULL, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: RestoreErasedContact :exec
UPDATE contacts SET deleted_at = NULL, updated_at = @now WHERE workspace_id = @workspace_id AND id = @id;

-- name: DeleteContactIdentities :execrows
DELETE FROM contact_identities WHERE workspace_id = @workspace_id AND contact_id = @contact_id;

-- name: FindPlaceholderContact :one
SELECT * FROM contacts WHERE workspace_id = @workspace_id AND placeholder;

-- name: CreatePlaceholderContact :one
-- CreatePlaceholderContact makes the workspace's one stand-in for erased
-- customers, hidden from every list, unless another request made it first.
INSERT INTO contacts (workspace_id, kind, name, placeholder, deleted_at, created_at, updated_at)
VALUES (@workspace_id, 'lead', 'Deleted contact', true, @now, @now, @now)
ON CONFLICT (workspace_id) WHERE placeholder DO UPDATE SET updated_at = contacts.updated_at
RETURNING *;

-- name: ListErasedMessageFiles :many
-- ListErasedMessageFiles returns the stored files of the messages in the
-- contact's conversations.
SELECT a.id, a.disk, a.path FROM attachments a
JOIN messages m ON m.id = a.message_id AND m.workspace_id = a.workspace_id
JOIN conversations c ON c.id = m.conversation_id AND c.workspace_id = m.workspace_id
WHERE c.workspace_id = @workspace_id AND c.contact_id = @contact_id;

-- name: DeleteErasedMessageFiles :execrows
DELETE FROM attachments a
USING messages m, conversations c
WHERE a.workspace_id = @workspace_id AND m.id = a.message_id AND m.workspace_id = a.workspace_id
  AND c.id = m.conversation_id AND c.workspace_id = m.workspace_id AND c.contact_id = @contact_id;

-- name: WipeErasedMessages :execrows
-- WipeErasedMessages empties every message of the contact's conversations,
-- keeping the rows so the timeline still shows that something was said.
UPDATE messages m
SET body = NULL, body_html = NULL, metadata = '{}', deleted_at = coalesce(m.deleted_at, @now::timestamptz),
    updated_at = @now::timestamptz
FROM conversations c
WHERE m.workspace_id = @workspace_id AND c.id = m.conversation_id AND c.workspace_id = m.workspace_id
  AND c.contact_id = @contact_id AND (m.body IS NOT NULL OR m.body_html IS NOT NULL);

-- name: HandOverErasedConversations :execrows
-- HandOverErasedConversations clears the subject, source and the agent's
-- memory of the contact's conversations and gives them to the placeholder.
UPDATE conversations
SET contact_id = @placeholder_id, subject = NULL, source = '{}', ai_memory = NULL, updated_at = @now
WHERE workspace_id = @workspace_id AND contact_id = @contact_id;

-- name: DeleteErasedContact :execrows
DELETE FROM contacts WHERE workspace_id = @workspace_id AND id = @id AND NOT placeholder;

-- name: ListErasedExportFiles :many
-- ListErasedExportFiles returns the zips of the contact's exports.
SELECT id, disk, path FROM contact_exports
WHERE workspace_id = @workspace_id AND contact_id = @contact_id AND disk IS NOT NULL AND path IS NOT NULL;

-- name: ScrubErasedRatingComments :execrows
-- ScrubErasedRatingComments forgets what an erased customer wrote about
-- articles. The answers stay, keyed by a hash that names nobody, so the
-- article's counts do not change.
UPDATE article_ratings SET comment = NULL, updated_at = @now
WHERE workspace_id = @workspace_id AND contact_id = @contact_id AND comment IS NOT NULL;

-- name: DeleteErasedChangelogSubscriptions :execrows
-- DeleteErasedChangelogSubscriptions removes the customer's changelog
-- subscriptions, those made under their address before they were known
-- included, so no update is ever mailed to them again.
DELETE FROM changelog_subscribers s
USING contacts c
WHERE s.workspace_id = @workspace_id AND c.workspace_id = s.workspace_id AND c.id = @contact_id
  AND (s.contact_id = c.id OR (c.email IS NOT NULL AND s.email = c.email));

-- name: ScrubErasedBroadcastRecipients :execrows
-- ScrubErasedBroadcastRecipients forgets the address and delivery details
-- broadcasts kept for the customer. The rows stay so a broadcast's counts
-- do not change.
UPDATE broadcast_recipients
SET address = '', reason = NULL, external_ref = NULL, message_id_header = NULL, updated_at = @now
WHERE workspace_id = @workspace_id AND contact_id = @contact_id
  AND (address <> '' OR reason IS NOT NULL OR external_ref IS NOT NULL OR message_id_header IS NOT NULL);

-- name: FindOpenContactErasure :one
-- FindOpenContactErasure returns the request under way for the contact,
-- which holds the identities they were known by.
SELECT * FROM contact_erasures
WHERE workspace_id = @workspace_id AND contact_id = @contact_id AND status IN ('pending', 'pending_hold');

-- name: FindErasedContactEmail :one
-- FindErasedContactEmail returns the address on the contact's profile.
SELECT email::text AS email FROM contacts WHERE workspace_id = @workspace_id AND id = @id;

-- name: ListErasedRawEmails :many
-- ListErasedRawEmails returns the stored source of the mail the contact
-- sent: mail in their conversations, and mail from their addresses that
-- never became a message.
SELECT i.id, i.raw_disk, i.raw_path FROM inbound_emails i
WHERE i.workspace_id = @workspace_id AND i.raw_path IS NOT NULL
  AND (
      i.conversation_id IN (
          SELECT c.id FROM conversations c WHERE c.workspace_id = @workspace_id AND c.contact_id = @contact_id
      )
      OR i.from_address = ANY (@emails::citext[])
      OR lower(i.envelope_from) = ANY (@emails::text[])
  );

-- name: DeleteErasedInboundEmails :execrows
-- DeleteErasedInboundEmails deletes the records of the mail the contact
-- sent.
DELETE FROM inbound_emails i
WHERE i.workspace_id = @workspace_id
  AND (
      i.conversation_id IN (
          SELECT c.id FROM conversations c WHERE c.workspace_id = @workspace_id AND c.contact_id = @contact_id
      )
      OR i.from_address = ANY (@emails::citext[])
      OR lower(i.envelope_from) = ANY (@emails::text[])
  );

-- name: ScrubErasedOutboundEmails :execrows
-- ScrubErasedOutboundEmails forgets who the mail of the contact's
-- conversations, and any other mail addressed to them, went to and what
-- it was about. The rows stay, with how the send ended, so delivery
-- counts do not change.
UPDATE outbound_emails o
SET to_addresses = '{}', cc_addresses = '{}', bcc_addresses = '{}', subject = '', recipient_statuses = '{}',
    last_error = NULL, metadata = '{}', updated_at = @now::timestamptz
WHERE o.workspace_id = @workspace_id
  AND (
      o.conversation_id IN (
          SELECT c.id FROM conversations c WHERE c.workspace_id = @workspace_id AND c.contact_id = @contact_id
      )
      OR lower(o.to_addresses::text)::text[] && @emails::text[]
  )
  AND (o.to_addresses <> '{}' OR o.cc_addresses <> '{}' OR o.bcc_addresses <> '{}' OR o.subject <> ''
      OR o.recipient_statuses <> '{}' OR o.last_error IS NOT NULL OR o.metadata <> '{}');

-- name: StripErasedOutboundCopies :execrows
-- StripErasedOutboundCopies takes the contact's addresses out of mail
-- they were only copied on, which stays as it was for its recipients.
UPDATE outbound_emails o
SET cc_addresses = ARRAY(SELECT a FROM unnest(o.cc_addresses) AS a WHERE lower(a) <> ALL (@emails::text[])),
    bcc_addresses = ARRAY(SELECT a FROM unnest(o.bcc_addresses) AS a WHERE lower(a) <> ALL (@emails::text[])),
    recipient_statuses = (
        SELECT coalesce(jsonb_object_agg(s.key, s.value), '{}'::jsonb)
        FROM jsonb_each(o.recipient_statuses) AS s
        WHERE lower(s.key) <> ALL (@emails::text[])
    ),
    updated_at = @now::timestamptz
WHERE o.workspace_id = @workspace_id
  AND (lower(o.cc_addresses::text)::text[] && @emails::text[] OR lower(o.bcc_addresses::text)::text[] && @emails::text[]);
