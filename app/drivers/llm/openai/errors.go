/*
|--------------------------------------------------------------------------
| OpenAI Error Mapping
|--------------------------------------------------------------------------
|
| One place where the provider's dialect becomes the shared kinds, so
| retries, key health and the gate never read a provider message. What
| is kept is the provider's code and a short message; a raw SDK error
| carries the request URL and sometimes the body, so it never lands on
| a step record.
|
*/

package openai

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	oai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/packages/ssestream"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/httpclient"

	"github.com/dimmihq/dimmi/app/contracts"
)

/*
maxMessage bounds the provider message kept on an error.
*/
const maxMessage = 300

/*
MapError folds an SDK error into a typed LLMError; now dates the
provider's retry hint.
*/
func MapError(err error, now time.Time) error {
	if err == nil {
		return nil
	}
	if llmErr, ok := errors.AsType[*contracts.LLMError](err); ok {
		return llmErr
	}
	if apiErr, ok := errors.AsType[*oai.Error](err); ok {
		return mapAPIError(apiErr, now)
	}
	if streamErr, ok := errors.AsType[*ssestream.StreamError](err); ok {
		return StreamFailure(streamErr.Event.Data)
	}
	return Transport(err)
}

/*
Transport maps a failure that never reached the provider's answer:
cancellation, a deadline, a refused host or a broken connection.
*/
func Transport(err error) error {
	switch {
	case errors.Is(err, context.Canceled):
		return &contracts.LLMError{Kind: contracts.LLMErrorCanceled, Message: "request canceled"}
	case errors.Is(err, context.DeadlineExceeded):
		return &contracts.LLMError{Kind: contracts.LLMErrorTimeout, Message: "request timed out"}
	case errors.Is(err, httpclient.ErrBlockedHost):
		return &contracts.LLMError{Kind: contracts.LLMErrorBlockedHost, Message: "provider host is not allowed"}
	}
	return &contracts.LLMError{Kind: contracts.LLMErrorNetwork, Message: "provider unreachable"}
}

/*
mapAPIError reads the typed SDK error for status, code and headers.
*/
func mapAPIError(err *oai.Error, now time.Time) error {
	code := err.Code
	if code == "" {
		code = err.Type
	}
	out := &contracts.LLMError{
		Kind:         KindForStatus(err.StatusCode, code),
		HTTPStatus:   err.StatusCode,
		ProviderCode: code,
		Message:      Short(err.Message),
	}
	if err.Response != nil {
		out.RetryAfter = retryAfter(err.Response.Header, now)
		out.RequestID = err.Response.Header.Get("x-request-id")
	}
	return out
}

/*
KindForStatus folds a status and the provider's error code into a
kind. A code naming an exhausted balance or plan wins over the
status, since such a 429 never clears with a retry.
*/
func KindForStatus(status int, code string) contracts.LLMErrorKind {
	if kind, ok := kindForCode(code); ok && kind == contracts.LLMErrorQuota {
		return kind
	}
	switch status {
	case fhttp.StatusUnauthorized:
		return contracts.LLMErrorAuth
	case fhttp.StatusForbidden:
		return contracts.LLMErrorPermission
	case fhttp.StatusNotFound:
		return contracts.LLMErrorModelNotFound
	case fhttp.StatusBadRequest, fhttp.StatusUnprocessableEntity, fhttp.StatusRequestEntityTooLarge:
		return contracts.LLMErrorInvalidRequest
	case fhttp.StatusPaymentRequired:
		return contracts.LLMErrorQuota
	case fhttp.StatusTooManyRequests:
		return contracts.LLMErrorRateLimited
	case fhttp.StatusRequestTimeout, fhttp.StatusGatewayTimeout:
		return contracts.LLMErrorTimeout
	case fhttp.StatusServiceUnavailable, 529:
		return contracts.LLMErrorOverloaded
	}
	if status >= 400 && status < 500 && status != fhttp.StatusConflict {
		return contracts.LLMErrorInvalidRequest
	}
	return contracts.LLMErrorServer
}

/*
kindForCode reads the error codes OpenAI-shaped providers send inside
a body or a stream event.
*/
func kindForCode(code string) (contracts.LLMErrorKind, bool) {
	switch code {
	case "insufficient_quota", "credit_balance_exhausted", "billing_error",
		"usage_limit_reached", "usage_not_included", "billing_hard_limit_reached":
		return contracts.LLMErrorQuota, true
	case "rate_limit_exceeded", "rate_limit_error":
		return contracts.LLMErrorRateLimited, true
	case "server_error", "api_error", "internal_error":
		return contracts.LLMErrorServer, true
	case "overloaded", "overloaded_error", "server_is_overloaded", "slow_down":
		return contracts.LLMErrorOverloaded, true
	case "invalid_request_error", "invalid_prompt", "context_length_exceeded":
		return contracts.LLMErrorInvalidRequest, true
	case "model_not_found":
		return contracts.LLMErrorModelNotFound, true
	case "invalid_api_key", "authentication_error":
		return contracts.LLMErrorAuth, true
	case "content_filter", "content_policy_violation":
		return contracts.LLMErrorContentFiltered, true
	}
	return "", false
}

/*
StreamFailure maps an error the provider sent inside an open stream,
shaped {"error": {"code", "type", "message"}} or flat.
*/
func StreamFailure(data []byte) error {
	var body struct {
		Error *struct {
			Code    any    `json:"code"`
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	_ = json.Unmarshal(data, &body)

	code, message := body.Code, body.Message
	if body.Error != nil {
		code, message = body.Error.Type, body.Error.Message
		if s, ok := body.Error.Code.(string); ok && s != "" {
			code = s
		}
	}
	return CodeFailure(code, message)
}

/*
CodeFailure builds the error for a provider code reported without an
HTTP status, as inside a stream.
*/
func CodeFailure(code, message string) error {
	kind, ok := kindForCode(code)
	if !ok {
		kind = contracts.LLMErrorServer
	}
	return &contracts.LLMError{Kind: kind, ProviderCode: code, Message: Short(message)}
}

/*
Short trims a provider message to one bounded line.
*/
func Short(message string) string {
	message = strings.Join(strings.Fields(message), " ")
	if len(message) <= maxMessage {
		return message
	}
	return message[:maxMessage] + "…"
}

/*
retryAfter reads the provider's retry hint in milliseconds or seconds.
*/
func retryAfter(h fhttp.Header, now time.Time) time.Duration {
	if v := h.Get("retry-after-ms"); v != "" {
		if ms, err := time.ParseDuration(strings.TrimSpace(v) + "ms"); err == nil && ms > 0 {
			return ms
		}
	}
	return httpclient.RetryAfter(h.Get("Retry-After"), now)
}
