/*
|--------------------------------------------------------------------------
| Anthropic Error Mapping
|--------------------------------------------------------------------------
|
| Anthropic's dialect becomes the shared kinds here, so retries and
| key health never read a provider message. The error type in the body
| decides first, because an error sent inside an open stream arrives
| with the stream's 200 status; a spend or credit limit is a quota,
| never a throttle, since waiting does not lift it. Only the provider's
| type and a short message are kept: the SDK's own error text carries
| the request URL.
|
*/

package anthropic

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	anth "github.com/anthropics/anthropic-sdk-go"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/httpclient"

	"github.com/dimmihq/dimmi/app/contracts"
)

/*
maxMessage bounds the provider message kept on an error.
*/
const maxMessage = 300

/*
streamErrorPrefix starts the SDK's text for a stream error it could
not type.
*/
const streamErrorPrefix = "received error while streaming: "

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
	if apiErr, ok := errors.AsType[*anth.Error](err); ok {
		return mapAPIError(apiErr, now)
	}
	if data, ok := strings.CutPrefix(err.Error(), streamErrorPrefix); ok {
		errType, message := readBody(data)
		return build(0, errType, message)
	}

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
mapAPIError reads the typed SDK error.
*/
func mapAPIError(err *anth.Error, now time.Time) error {
	errType, message := readBody(err.RawJSON())
	if t := string(err.Type()); t != "" {
		errType = t
	}

	status := err.StatusCode
	if status < 300 {
		status = 0
	}
	out := build(status, errType, message)
	out.RequestID = err.RequestID
	if err.Response != nil {
		out.RetryAfter = httpclient.RetryAfter(err.Response.Header.Get("Retry-After"), now)
	}
	return out
}

/*
build assembles the error from the status, the body's type and its
message.
*/
func build(status int, errType, message string) *contracts.LLMError {
	kind, ok := kindForType(errType)
	if !ok {
		kind = kindForStatus(status)
	}
	if spendLimit(kind, message) {
		kind = contracts.LLMErrorQuota
	}
	return &contracts.LLMError{Kind: kind, HTTPStatus: status, ProviderCode: errType, Message: short(message)}
}

/*
kindForType maps Anthropic's error types.
*/
func kindForType(errType string) (contracts.LLMErrorKind, bool) {
	switch errType {
	case "authentication_error":
		return contracts.LLMErrorAuth, true
	case "permission_error":
		return contracts.LLMErrorPermission, true
	case "not_found_error":
		return contracts.LLMErrorModelNotFound, true
	case "invalid_request_error", "request_too_large":
		return contracts.LLMErrorInvalidRequest, true
	case "billing_error":
		return contracts.LLMErrorQuota, true
	case "rate_limit_error":
		return contracts.LLMErrorRateLimited, true
	case "timeout_error":
		return contracts.LLMErrorTimeout, true
	case "overloaded_error":
		return contracts.LLMErrorOverloaded, true
	case "api_error":
		return contracts.LLMErrorServer, true
	}
	return "", false
}

/*
kindForStatus maps a status when the body named no type.
*/
func kindForStatus(status int) contracts.LLMErrorKind {
	switch status {
	case fhttp.StatusUnauthorized:
		return contracts.LLMErrorAuth
	case fhttp.StatusForbidden:
		return contracts.LLMErrorPermission
	case fhttp.StatusNotFound:
		return contracts.LLMErrorModelNotFound
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
spendLimit reports a throttle or refusal that is really an exhausted
credit balance or a reached spend limit.
*/
func spendLimit(kind contracts.LLMErrorKind, message string) bool {
	if kind != contracts.LLMErrorRateLimited && kind != contracts.LLMErrorInvalidRequest {
		return false
	}
	lower := strings.ToLower(message)
	for _, phrase := range []string{"credit balance", "usage limit", "spend limit", "spending limit", "billing"} {
		if strings.Contains(lower, phrase) {
			return true
		}
	}
	return false
}

/*
readBody reads {"error": {"type", "message"}} from a response body.
*/
func readBody(raw string) (errType, message string) {
	var body struct {
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal([]byte(raw), &body) != nil {
		return "", ""
	}
	return body.Error.Type, body.Error.Message
}

/*
short trims a provider message to one bounded line.
*/
func short(message string) string {
	message = strings.Join(strings.Fields(message), " ")
	if len(message) <= maxMessage {
		return message
	}
	return message[:maxMessage] + "…"
}
