-- name: ListFeedbackBoards :many
-- ListFeedbackBoards returns a help center's boards in the team's
-- order.
SELECT * FROM feedback_boards
WHERE workspace_id = @workspace_id AND help_center_id = @help_center_id
ORDER BY position, created_at, id;

-- name: FindFeedbackBoard :one
-- FindFeedbackBoard returns one board.
SELECT * FROM feedback_boards WHERE workspace_id = @workspace_id AND id = @id;

-- name: LockFeedbackBoard :one
-- LockFeedbackBoard returns one board and holds it until the
-- transaction ends.
SELECT * FROM feedback_boards WHERE workspace_id = @workspace_id AND id = @id FOR UPDATE;

-- name: FindFeedbackBoardBySlug :one
-- FindFeedbackBoardBySlug returns a help center's board by slug.
SELECT * FROM feedback_boards
WHERE workspace_id = @workspace_id AND help_center_id = @help_center_id AND slug = @slug;

-- name: FeedbackBoardSlugTaken :one
-- FeedbackBoardSlugTaken says whether another board of the help center
-- uses the slug.
SELECT EXISTS (
    SELECT 1 FROM feedback_boards
    WHERE help_center_id = @help_center_id AND slug = @slug AND id <> @id
) AS taken;

-- name: CreateFeedbackBoard :one
-- CreateFeedbackBoard stores a board after the help center's others.
INSERT INTO feedback_boards (workspace_id, help_center_id, name, slug, description, visibility, approval_required,
    position, created_at, updated_at)
VALUES (@workspace_id, @help_center_id, @name, @slug, @description, @visibility, @approval_required,
    (SELECT coalesce(max(position) + 1, 0) FROM feedback_boards
        WHERE workspace_id = @workspace_id AND help_center_id = @help_center_id),
    @now, @now)
ON CONFLICT (help_center_id, slug) DO NOTHING
RETURNING *;

-- name: UpdateFeedbackBoard :one
-- UpdateFeedbackBoard rewrites a board.
UPDATE feedback_boards SET name = @name, slug = @slug, description = @description, visibility = @visibility,
    approval_required = @approval_required, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: DeleteFeedbackBoard :exec
-- DeleteFeedbackBoard deletes a board that holds no requests.
DELETE FROM feedback_boards WHERE workspace_id = @workspace_id AND id = @id;

-- name: SaveFeedbackBoardPositions :exec
-- SaveFeedbackBoardPositions stores the help center's boards in the
-- order given.
UPDATE feedback_boards b SET position = (s.ord - 1)::integer, updated_at = @now
FROM unnest(@board_ids::uuid[]) WITH ORDINALITY AS s(id, ord)
WHERE b.workspace_id = @workspace_id AND b.help_center_id = @help_center_id AND b.id = s.id;

-- name: MoveFeedbackPostsToBoard :exec
-- MoveFeedbackPostsToBoard moves every request of a board, deleted ones
-- included, onto another.
UPDATE feedback_posts SET board_id = @target_id, updated_at = @now
WHERE workspace_id = @workspace_id AND board_id = @source_id;

-- name: AdoptFeedbackPosts :exec
-- AdoptFeedbackPosts puts a help center's requests that have no board
-- on the board given.
UPDATE feedback_posts SET board_id = @board_id
WHERE workspace_id = @workspace_id AND help_center_id = @help_center_id AND board_id IS NULL;

-- name: CountFeedbackByBoard :many
-- CountFeedbackByBoard counts each board's live requests that were not
-- merged away.
SELECT board_id, count(*)::integer AS posts FROM feedback_posts
WHERE workspace_id = @workspace_id AND help_center_id = @help_center_id AND deleted_at IS NULL
  AND merged_into_id IS NULL
GROUP BY board_id;

-- name: SetFeedbackPostBoard :one
-- SetFeedbackPostBoard moves one request onto a board.
UPDATE feedback_posts SET board_id = @board_id, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL
RETURNING *;
