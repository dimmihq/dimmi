-- name: FindPendingUpload :one
-- FindPendingUpload returns the upload with the id when the uploader named
-- made it and no message took it yet, finished or not.
SELECT * FROM attachments
WHERE workspace_id = @workspace_id AND id = @id AND uploaded_by_type = @uploaded_by_type
  AND uploaded_by_id = @uploaded_by_id::uuid AND message_id IS NULL;

-- name: CompleteUpload :one
-- CompleteUpload marks the upload's bytes as arrived, with what checking
-- them found and their checksum. An upload already complete is left as it
-- was.
UPDATE attachments
SET uploaded_at = @now, content_type = @content_type, width = sqlc.narg(width), height = sqlc.narg(height),
    sha256 = @sha256, scan_status = @scan_status, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND uploaded_at IS NULL
RETURNING *;
