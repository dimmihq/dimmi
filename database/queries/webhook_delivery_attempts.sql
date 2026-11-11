-- name: CreateWebhookDeliveryAttempt :one
-- CreateWebhookDeliveryAttempt records one request sent and what came back.
INSERT INTO webhook_delivery_attempts (
    workspace_id, delivery_id, number, trigger, requested_at, duration_ms, request_headers, response_status,
    response_headers, response_body, error_code, error_message, created_at
) VALUES (
    @workspace_id, @delivery_id, @number, @trigger, @requested_at, @duration_ms, @request_headers,
    sqlc.narg(response_status), sqlc.narg(response_headers), sqlc.narg(response_body), sqlc.narg(error_code),
    sqlc.narg(error_message), @requested_at
)
RETURNING *;

-- name: ListWebhookDeliveryAttempts :many
-- ListWebhookDeliveryAttempts returns a delivery's attempts, first one first.
SELECT * FROM webhook_delivery_attempts
WHERE workspace_id = @workspace_id AND delivery_id = @delivery_id
ORDER BY number;

-- name: NextWebhookAttemptNumber :one
-- NextWebhookAttemptNumber returns the number the delivery's next attempt takes.
SELECT (coalesce(max(number), 0) + 1)::integer AS number FROM webhook_delivery_attempts
WHERE workspace_id = @workspace_id AND delivery_id = @delivery_id;
