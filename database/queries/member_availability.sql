-- name: SetMemberAvailability :one
-- SetMemberAvailability stores how available the member is for new
-- conversations, whether replying takes one, and how many they hold.
UPDATE workspace_members
SET availability = @availability, away_reassign = @away_reassign, away_since = sqlc.narg(away_since),
    availability_source = @availability_source, assign_on_reply = @assign_on_reply,
    conversation_capacity = sqlc.narg(conversation_capacity), capacity_by_channel = @capacity_by_channel,
    updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;
