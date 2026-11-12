-- name: NextFeedbackNumber :one
-- NextFeedbackNumber is the number the workspace's next post takes.
SELECT (coalesce(max(number), 0) + 1)::integer AS number FROM feedback_posts WHERE workspace_id = @workspace_id;

-- name: CreateFeedbackPost :one
-- CreateFeedbackPost stores a new post.
INSERT INTO feedback_posts (
    workspace_id, help_center_id, board_id, number, title, slug, body_markdown, status, moderation,
    author_contact_id, author_member_id, created_at, updated_at
) VALUES (
    @workspace_id, @help_center_id, @board_id, @number, @title, @slug, @body_markdown, @status, @moderation,
    @author_contact_id, @author_member_id, @now, @now
)
RETURNING *;

-- name: FindFeedbackPost :one
-- FindFeedbackPost returns one live post.
SELECT * FROM feedback_posts WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL;

-- name: LockFeedbackPost :one
-- LockFeedbackPost returns the post and holds it until the transaction
-- ends.
SELECT * FROM feedback_posts WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL FOR UPDATE;

-- name: FindFeedbackPostByNumber :one
-- FindFeedbackPostByNumber returns a help center's live post by number.
SELECT * FROM feedback_posts
WHERE workspace_id = @workspace_id AND help_center_id = @help_center_id AND number = @number AND deleted_at IS NULL;

-- name: UpdateFeedbackPost :one
-- UpdateFeedbackPost rewrites a post's title and details.
UPDATE feedback_posts SET title = @title, slug = @slug, body_markdown = @body_markdown, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL
RETURNING *;

-- name: SetFeedbackStatus :one
-- SetFeedbackStatus moves a post to a status, with the changelog update
-- that shipped it when there is one.
UPDATE feedback_posts SET status = @status, changelog_entry_id = @changelog_entry_id, status_changed_at = @now,
    updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL
RETURNING *;

-- name: SetFeedbackModeration :one
-- SetFeedbackModeration approves a post or marks it spam.
UPDATE feedback_posts SET moderation = @moderation, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL
RETURNING *;

-- name: MergeFeedbackPost :one
-- MergeFeedbackPost points a post at the one it duplicates.
UPDATE feedback_posts SET merged_into_id = @target_id, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL
RETURNING *;

-- name: DeleteFeedbackPost :exec
-- DeleteFeedbackPost deletes a post.
UPDATE feedback_posts SET deleted_at = @now, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL;

-- name: ListFeedbackPosts :many
-- ListFeedbackPosts returns a page of a help center's posts that were
-- not merged away, narrowed by board, the boards a reader may see,
-- status, moderation and title when given,
-- in the order asked for: trending, top or new.
SELECT * FROM feedback_posts
WHERE workspace_id = @workspace_id AND help_center_id = @help_center_id AND deleted_at IS NULL
  AND merged_into_id IS NULL
  AND (sqlc.narg(board_id)::uuid IS NULL OR board_id = sqlc.narg(board_id)::uuid)
  AND (sqlc.narg(board_ids)::uuid[] IS NULL OR board_id = ANY(sqlc.narg(board_ids)::uuid[]))
  AND (sqlc.narg(status)::text IS NULL OR status = sqlc.narg(status)::text)
  AND (sqlc.narg(moderation)::text IS NULL OR moderation = sqlc.narg(moderation)::text)
  AND (sqlc.narg(query)::text IS NULL OR title_trgm LIKE '%' || lower(dimmi_unaccent(sqlc.narg(query)::text)) || '%')
ORDER BY
    CASE WHEN @sort::text = 'top' THEN vote_count END DESC,
    CASE WHEN @sort::text = 'trending' THEN coalesce(trending_log, -1e9) END DESC,
    created_at DESC, id DESC
LIMIT @row_limit OFFSET @row_offset;

-- name: CountFeedbackPosts :one
-- CountFeedbackPosts counts what ListFeedbackPosts would list.
SELECT count(*) FROM feedback_posts
WHERE workspace_id = @workspace_id AND help_center_id = @help_center_id AND deleted_at IS NULL
  AND merged_into_id IS NULL
  AND (sqlc.narg(board_id)::uuid IS NULL OR board_id = sqlc.narg(board_id)::uuid)
  AND (sqlc.narg(board_ids)::uuid[] IS NULL OR board_id = ANY(sqlc.narg(board_ids)::uuid[]))
  AND (sqlc.narg(status)::text IS NULL OR status = sqlc.narg(status)::text)
  AND (sqlc.narg(moderation)::text IS NULL OR moderation = sqlc.narg(moderation)::text)
  AND (sqlc.narg(query)::text IS NULL OR title_trgm LIKE '%' || lower(dimmi_unaccent(sqlc.narg(query)::text)) || '%');

-- name: CountFeedbackByStatus :many
-- CountFeedbackByStatus counts a help center's approved posts in each
-- status, and the ones waiting for approval under 'pending', on one
-- board when given.
SELECT (CASE WHEN moderation = 'approved' THEN status ELSE moderation END)::text AS bucket, count(*)::integer AS posts
FROM feedback_posts
WHERE workspace_id = @workspace_id AND help_center_id = @help_center_id AND deleted_at IS NULL
  AND merged_into_id IS NULL
  AND (sqlc.narg(board_id)::uuid IS NULL OR board_id = sqlc.narg(board_id)::uuid)
GROUP BY 1;

-- name: SimilarFeedbackPosts :many
-- SimilarFeedbackPosts finds approved posts whose titles look like the
-- text, closest first, on the boards given when there are any.
SELECT * FROM feedback_posts
WHERE workspace_id = @workspace_id AND help_center_id = @help_center_id AND deleted_at IS NULL
  AND merged_into_id IS NULL AND moderation = 'approved'
  AND (sqlc.narg(board_ids)::uuid[] IS NULL OR board_id = ANY(sqlc.narg(board_ids)::uuid[]))
  AND similarity(title_trgm, lower(dimmi_unaccent(@query::text))) > 0.25
ORDER BY similarity(title_trgm, lower(dimmi_unaccent(@query::text))) DESC
LIMIT 5;

-- name: AddFeedbackVote :execrows
-- AddFeedbackVote counts a customer's vote once.
INSERT INTO feedback_votes (post_id, contact_id, workspace_id, member_id, priority, created_at)
VALUES (@post_id, @contact_id, @workspace_id, @member_id, @priority, @now)
ON CONFLICT (post_id, contact_id) DO UPDATE SET priority = excluded.priority,
    member_id = coalesce(feedback_votes.member_id, excluded.member_id);

-- name: RemoveFeedbackVote :execrows
-- RemoveFeedbackVote takes a customer's vote back.
DELETE FROM feedback_votes WHERE workspace_id = @workspace_id AND post_id = @post_id AND contact_id = @contact_id;

-- name: RecountFeedbackPost :one
-- RecountFeedbackPost recounts a post's votes and public comments, and
-- its trending score: the log2 of the sum of 2^((vote time - epoch) /
-- one week) over its votes.
UPDATE feedback_posts p SET
    vote_count = v.votes,
    trending_log = v.trend,
    comment_count = (SELECT count(*) FROM feedback_comments c
        WHERE c.post_id = p.id AND NOT c.internal AND c.deleted_at IS NULL)
FROM (
    SELECT count(*)::integer AS votes,
        CASE WHEN count(*) = 0 THEN NULL ELSE
            max(e) + log(2::numeric, sum(power(2::numeric, (e - mx)::numeric)))::double precision
        END AS trend
    FROM (
        SELECT extract(epoch FROM created_at - timestamptz '2026-01-01T00:00:00Z') / 604800.0 AS e,
            max(extract(epoch FROM created_at - timestamptz '2026-01-01T00:00:00Z') / 604800.0) OVER () AS mx
        FROM feedback_votes WHERE post_id = @id AND workspace_id = @workspace_id
    ) w
) v
WHERE p.workspace_id = @workspace_id AND p.id = @id
RETURNING p.*;

-- name: ListFeedbackVoters :many
-- ListFeedbackVoters returns a post's voters, newest first, with who
-- they are and the teammate who voted for them, if one did.
SELECT v.contact_id, v.priority, v.created_at, v.member_id, c.name, c.email, c.avatar_url, co.name AS company
FROM feedback_votes v
JOIN contacts c ON c.id = v.contact_id
LEFT JOIN companies co ON co.id = c.company_id
WHERE v.workspace_id = @workspace_id AND v.post_id = @post_id
ORDER BY v.created_at DESC
LIMIT 500;

-- name: ListVotedFeedbackPosts :many
-- ListVotedFeedbackPosts returns which of the posts the customer voted
-- for.
SELECT post_id FROM feedback_votes
WHERE workspace_id = @workspace_id AND contact_id = @contact_id AND post_id = ANY(@post_ids::uuid[]);

-- name: MoveFeedbackVotes :exec
-- MoveFeedbackVotes carries a post's votes over to another, once per
-- customer, and removes them from the first.
WITH moved AS (
    DELETE FROM feedback_votes d WHERE d.workspace_id = @workspace_id AND d.post_id = @source_id
    RETURNING d.contact_id, d.workspace_id, d.member_id, d.priority, d.created_at
)
INSERT INTO feedback_votes (post_id, contact_id, workspace_id, member_id, priority, created_at)
SELECT @target_id, m.contact_id, m.workspace_id, m.member_id, m.priority, m.created_at FROM moved m
ON CONFLICT (post_id, contact_id) DO NOTHING;

-- name: MoveFeedbackComments :exec
-- MoveFeedbackComments carries a post's comments over to another.
UPDATE feedback_comments SET post_id = @target_id WHERE workspace_id = @workspace_id AND post_id = @source_id;

-- name: ListFeedbackRecipients :many
-- ListFeedbackRecipients returns the addresses to tell about a post: its
-- voters, its author and the customers of its linked conversations, once
-- each.
SELECT DISTINCT c.id, c.email, c.name FROM contacts c
WHERE c.workspace_id = @workspace_id AND c.email IS NOT NULL AND (
    c.id IN (SELECT fv.contact_id FROM feedback_votes fv WHERE fv.workspace_id = @workspace_id AND fv.post_id = @post_id)
    OR c.id IN (SELECT fl.contact_id FROM feedback_post_conversations fl
        WHERE fl.workspace_id = @workspace_id AND fl.post_id = @post_id)
    OR c.id = (SELECT fp.author_contact_id FROM feedback_posts fp WHERE fp.workspace_id = @workspace_id AND fp.id = @post_id)
);

-- name: CreateFeedbackComment :one
-- CreateFeedbackComment stores a reply or an internal note.
INSERT INTO feedback_comments (workspace_id, post_id, parent_id, author_contact_id, author_member_id, body, internal,
    created_at)
VALUES (@workspace_id, @post_id, @parent_id, @author_contact_id, @author_member_id, @body, @internal, @now)
RETURNING *;

-- name: FindFeedbackComment :one
-- FindFeedbackComment returns one live comment.
SELECT * FROM feedback_comments WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL;

-- name: DeleteFeedbackComment :exec
-- DeleteFeedbackComment deletes a comment.
UPDATE feedback_comments SET deleted_at = @now WHERE workspace_id = @workspace_id AND id = @id;

-- name: ListFeedbackComments :many
-- ListFeedbackComments returns a post's live comments, oldest first,
-- with who wrote each; internal notes only when asked for.
SELECT fc.id, fc.parent_id, fc.body, fc.internal, fc.created_at, fc.author_contact_id, fc.author_member_id,
    coalesce(u.name, c.name, c.email, 'Customer')::text AS author_name,
    coalesce(u.avatar_path, c.avatar_url) AS author_avatar
FROM feedback_comments fc
LEFT JOIN contacts c ON c.id = fc.author_contact_id
LEFT JOIN workspace_members m ON m.id = fc.author_member_id
LEFT JOIN users u ON u.id = m.user_id
WHERE fc.workspace_id = @workspace_id AND fc.post_id = @post_id AND fc.deleted_at IS NULL
  AND (@include_internal::boolean OR NOT fc.internal)
ORDER BY fc.created_at, fc.id;

-- name: FindFeedbackAuthorNames :many
-- FindFeedbackAuthorNames returns the names of the posts' authors: the
-- customer a request is for before the teammate who posted it.
SELECT p.id, coalesce(c.name, c.email, u.name, 'Customer')::text AS author_name
FROM feedback_posts p
LEFT JOIN contacts c ON c.id = p.author_contact_id
LEFT JOIN workspace_members m ON m.id = p.author_member_id
LEFT JOIN users u ON u.id = m.user_id
WHERE p.workspace_id = @workspace_id AND p.id = ANY(@post_ids::uuid[]);
