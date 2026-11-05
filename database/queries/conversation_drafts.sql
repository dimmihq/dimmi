-- name: ListMemberDrafts :many
-- ListMemberDrafts returns the member's drafts, newest first, those of
-- one conversation or one mode when asked, leaving out drafts of
-- conversations in the trash or that the member may no longer see.
SELECT d.* FROM conversation_drafts d
LEFT JOIN conversations c ON c.id = d.conversation_id
WHERE d.workspace_id = @workspace_id AND d.member_id = @member_id
  AND (sqlc.narg('conversation_id')::uuid IS NULL OR d.conversation_id = sqlc.narg('conversation_id')::uuid)
  AND (sqlc.narg('mode')::text IS NULL OR d.mode = sqlc.narg('mode')::text)
  AND (d.conversation_id IS NULL OR (c.deleted_at IS NULL AND dimmi_conversation_visible(
      c.assignee_member_id, c.assignee_team_id, @access_mode::text, sqlc.narg(access_viewer)::uuid,
      @access_teams::uuid[], @access_unassigned::boolean
  )))
ORDER BY d.updated_at DESC, d.id DESC
LIMIT @row_limit;

-- name: FindMemberDraft :one
-- FindMemberDraft returns one of the member's drafts.
SELECT * FROM conversation_drafts WHERE workspace_id = @workspace_id AND member_id = @member_id AND id = @id;

-- name: FindMemberDraftFor :one
-- FindMemberDraftFor returns the member's draft of the conversation and
-- mode, the conversation nil for a new conversation's draft.
SELECT * FROM conversation_drafts
WHERE workspace_id = @workspace_id AND member_id = @member_id
  AND conversation_id IS NOT DISTINCT FROM sqlc.narg('conversation_id')::uuid AND mode = @mode;

-- name: CreateDraft :one
-- CreateDraft saves a new draft.
INSERT INTO conversation_drafts (
    workspace_id, member_id, conversation_id, mode, body, attachment_ids, recipients, client_updated_at,
    created_at, updated_at
) VALUES (
    @workspace_id, @member_id, sqlc.narg('conversation_id'), @mode, @body, @attachment_ids::uuid[], @recipients,
    @client_updated_at, @now, @now
)
RETURNING *;

-- name: UpdateDraft :one
-- UpdateDraft writes the draft's content whole.
UPDATE conversation_drafts
SET body = @body, attachment_ids = @attachment_ids::uuid[], recipients = @recipients,
    client_updated_at = @client_updated_at, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: DeleteDraft :execrows
-- DeleteDraft removes one of the member's drafts.
DELETE FROM conversation_drafts WHERE workspace_id = @workspace_id AND member_id = @member_id AND id = @id;

-- name: DeleteDraftFor :many
-- DeleteDraftFor removes the member's draft of the conversation and mode
-- and returns its id, none when there was no draft.
DELETE FROM conversation_drafts
WHERE workspace_id = @workspace_id AND member_id = @member_id
  AND conversation_id IS NOT DISTINCT FROM sqlc.narg('conversation_id')::uuid AND mode = @mode
RETURNING id;

-- name: MoveConversationDrafts :many
-- MoveConversationDrafts hands the drafts of one conversation to another,
-- except where the member already has a draft of the same mode on the
-- target; those stay behind until the purge removes them. It returns the
-- drafts moved.
UPDATE conversation_drafts d
SET conversation_id = @target_id::uuid, updated_at = @now
WHERE d.workspace_id = @workspace_id AND d.conversation_id = @source_id::uuid
  AND NOT EXISTS (
      SELECT 1 FROM conversation_drafts t
      WHERE t.member_id = d.member_id AND t.conversation_id = @target_id::uuid AND t.mode = d.mode
  )
RETURNING d.*;

-- name: PurgeStaleDrafts :execrows
-- PurgeStaleDrafts deletes up to a batch of the workspace's drafts
-- untouched since the cutoff, oldest first.
DELETE FROM conversation_drafts
WHERE id IN (
    SELECT d.id FROM conversation_drafts AS d
    WHERE d.workspace_id = @workspace_id AND d.updated_at < @cutoff
    ORDER BY d.updated_at LIMIT @batch::bigint
);
