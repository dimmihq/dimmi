-- name: LinkFeedbackConversation :execrows
-- LinkFeedbackConversation links a conversation to a request, once.
INSERT INTO feedback_post_conversations (post_id, conversation_id, workspace_id, contact_id, member_id, created_at)
VALUES (@post_id, @conversation_id, @workspace_id, @contact_id, @member_id, @now)
ON CONFLICT (post_id, conversation_id) DO NOTHING;

-- name: UnlinkFeedbackConversation :execrows
-- UnlinkFeedbackConversation removes a conversation's link to a
-- request.
DELETE FROM feedback_post_conversations
WHERE workspace_id = @workspace_id AND post_id = @post_id AND conversation_id = @conversation_id;

-- name: ListConversationFeedbackPosts :many
-- ListConversationFeedbackPosts returns the live requests linked to a
-- conversation, newest link first.
SELECT p.* FROM feedback_post_conversations l
JOIN feedback_posts p ON p.id = l.post_id
WHERE l.workspace_id = @workspace_id AND l.conversation_id = @conversation_id AND p.deleted_at IS NULL
ORDER BY l.created_at DESC
LIMIT 50;

-- name: ListContactFeedbackPosts :many
-- ListContactFeedbackPosts returns the live requests a customer voted
-- for or was linked to, newest first.
SELECT p.* FROM feedback_posts p
WHERE p.workspace_id = @workspace_id AND p.deleted_at IS NULL AND p.merged_into_id IS NULL AND (
    p.id IN (SELECT v.post_id FROM feedback_votes v WHERE v.workspace_id = @workspace_id AND v.contact_id = @contact_id)
    OR p.id IN (SELECT l.post_id FROM feedback_post_conversations l
        WHERE l.workspace_id = @workspace_id AND l.contact_id = @contact_id)
)
ORDER BY p.updated_at DESC
LIMIT 50;

-- name: ListFeedbackPostConversations :many
-- ListFeedbackPostConversations returns the conversations linked to a
-- request that the reader may see, newest first, with their customer
-- and who linked them.
SELECT l.conversation_id, l.contact_id, l.member_id, l.created_at, cv.number, cv.subject, cv.status AS conversation_status,
    c.name AS contact_name, c.email AS contact_email, co.name AS company
FROM feedback_post_conversations l
JOIN conversations cv ON cv.id = l.conversation_id
LEFT JOIN contacts c ON c.id = l.contact_id
LEFT JOIN companies co ON co.id = c.company_id
WHERE l.workspace_id = @workspace_id AND l.post_id = @post_id AND cv.deleted_at IS NULL
  AND dimmi_conversation_visible(
      cv.assignee_member_id, cv.assignee_team_id, cv.inbox_id, @access_mode::text,
      sqlc.narg(access_viewer)::uuid, @access_teams::uuid[], @access_unassigned::boolean,
      @access_hidden_inboxes::uuid[]
  )
ORDER BY l.created_at DESC
LIMIT 200;

-- name: MoveFeedbackConversations :exec
-- MoveFeedbackConversations carries a request's linked conversations
-- over to another, once each.
WITH moved AS (
    DELETE FROM feedback_post_conversations d WHERE d.workspace_id = @workspace_id AND d.post_id = @source_id
    RETURNING d.conversation_id, d.workspace_id, d.contact_id, d.member_id, d.created_at
)
INSERT INTO feedback_post_conversations (post_id, conversation_id, workspace_id, contact_id, member_id, created_at)
SELECT @target_id, m.conversation_id, m.workspace_id, m.contact_id, m.member_id, m.created_at FROM moved m
ON CONFLICT (post_id, conversation_id) DO NOTHING;
