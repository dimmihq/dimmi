-- name: ListBroadcastTopics :many
-- ListBroadcastTopics returns the workspace's live topics by name.
SELECT * FROM broadcast_topics WHERE workspace_id = @workspace_id AND archived_at IS NULL ORDER BY lower(name), id;

-- name: FindBroadcastTopic :one
-- FindBroadcastTopic returns one live topic.
SELECT * FROM broadcast_topics WHERE workspace_id = @workspace_id AND id = @id AND archived_at IS NULL;

-- name: CreateBroadcastTopic :one
-- CreateBroadcastTopic stores a new topic.
INSERT INTO broadcast_topics (workspace_id, name, description, consent, created_at, updated_at)
VALUES (@workspace_id, @name, @description, @consent, @now::timestamptz, @now::timestamptz)
RETURNING *;

-- name: UpdateBroadcastTopic :one
-- UpdateBroadcastTopic renames, describes or archives a topic. Consent
-- never changes, since people chose under it.
UPDATE broadcast_topics SET name = @name, description = @description, archived_at = sqlc.narg(archived_at),
    updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id AND archived_at IS NULL
RETURNING *;

-- name: ListContactTopicSubscriptions :many
-- ListContactTopicSubscriptions returns the choices a contact made.
SELECT * FROM contact_topic_subscriptions WHERE workspace_id = @workspace_id AND contact_id = @contact_id;

-- name: SetContactTopicSubscription :exec
-- SetContactTopicSubscription records a contact's choice for a topic.
INSERT INTO contact_topic_subscriptions (workspace_id, contact_id, topic_id, subscribed, source, updated_at)
VALUES (@workspace_id, @contact_id, @topic_id, @subscribed, @source, @now::timestamptz)
ON CONFLICT (workspace_id, contact_id, topic_id)
DO UPDATE SET subscribed = EXCLUDED.subscribed, source = EXCLUDED.source, updated_at = EXCLUDED.updated_at;

-- name: OptOutOfBroadcasts :exec
-- OptOutOfBroadcasts stops every broadcast but service notices to the
-- contact.
INSERT INTO broadcast_opt_outs (workspace_id, contact_id, source, created_at)
VALUES (@workspace_id, @contact_id, @source, @now::timestamptz)
ON CONFLICT (workspace_id, contact_id) DO NOTHING;

-- name: OptBackIntoBroadcasts :exec
-- OptBackIntoBroadcasts lifts a contact's opt-out of all broadcasts.
DELETE FROM broadcast_opt_outs WHERE workspace_id = @workspace_id AND contact_id = @contact_id;

-- name: ContactOptedOutOfBroadcasts :one
-- ContactOptedOutOfBroadcasts reports whether the contact left all
-- broadcasts.
SELECT EXISTS (SELECT 1 FROM broadcast_opt_outs WHERE workspace_id = @workspace_id AND contact_id = @contact_id);

-- name: BroadcastTopicTallies :many
-- BroadcastTopicTallies counts, for each live topic, the people who
-- said yes, the people who said no and the broadcasts sent under it.
SELECT t.id,
    (SELECT count(*) FROM contact_topic_subscriptions s JOIN contacts c ON c.id = s.contact_id
        WHERE s.topic_id = t.id AND s.subscribed AND c.deleted_at IS NULL AND c.merged_into_id IS NULL)::bigint AS said_yes,
    (SELECT count(*) FROM contact_topic_subscriptions s JOIN contacts c ON c.id = s.contact_id
        WHERE s.topic_id = t.id AND NOT s.subscribed AND c.deleted_at IS NULL AND c.merged_into_id IS NULL)::bigint AS said_no,
    (SELECT count(*) FROM broadcasts b WHERE b.topic_id = t.id AND b.deleted_at IS NULL)::bigint AS broadcasts
FROM broadcast_topics t
WHERE t.workspace_id = @workspace_id AND t.archived_at IS NULL;

-- name: CountBroadcastPeople :one
-- CountBroadcastPeople counts the people broadcasts may reach: everyone
-- who has not left all broadcasts.
SELECT count(*)::bigint FROM contacts c
WHERE c.workspace_id = @workspace_id AND c.deleted_at IS NULL AND c.merged_into_id IS NULL
    AND NOT EXISTS (SELECT 1 FROM broadcast_opt_outs o WHERE o.workspace_id = c.workspace_id AND o.contact_id = c.id);
