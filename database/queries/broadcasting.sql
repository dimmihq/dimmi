-- name: FindBroadcastMessage :one
-- FindBroadcastMessage returns the message with the id in the workspace,
-- deleted or not, as the realtime broadcaster shows it to open screens.
SELECT * FROM messages WHERE workspace_id = @workspace_id AND id = @id;
