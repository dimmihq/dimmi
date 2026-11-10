/*
|--------------------------------------------------------------------------
| LLM Errors
|--------------------------------------------------------------------------
|
| Provider failures arrive in every dialect; the pipeline only speaks
| one. Each driver maps its SDK's errors to one kind here, and the
| retry layer, the key health checks and the gate all branch on that
| kind alone. A message never carries a key or prompt text.
|
*/

package contracts

import (
	"errors"
	"time"
)

/*
LLMErrorKind names the class of a provider failure.
*/
type LLMErrorKind string

/*
The failure kinds, and whether a retry is ever worthwhile. Retry lives
in the retry wrapper, never in a driver.
*/
const (
	LLMErrorAuth            LLMErrorKind = "auth"
	LLMErrorPermission      LLMErrorKind = "permission"
	LLMErrorQuota           LLMErrorKind = "quota"
	LLMErrorRateLimited     LLMErrorKind = "rate_limited"
	LLMErrorOverloaded      LLMErrorKind = "overloaded"
	LLMErrorServer          LLMErrorKind = "server"
	LLMErrorTimeout         LLMErrorKind = "timeout"
	LLMErrorInvalidRequest  LLMErrorKind = "invalid_request"
	LLMErrorModelNotFound   LLMErrorKind = "model_not_found"
	LLMErrorContentFiltered LLMErrorKind = "content_filtered"
	LLMErrorNetwork         LLMErrorKind = "network"
	LLMErrorBlockedHost     LLMErrorKind = "blocked_host"
	LLMErrorCanceled        LLMErrorKind = "canceled"
)

/*
LLMError is the one error the pipeline reads from a driver.
*/
type LLMError struct {
	Kind         LLMErrorKind
	HTTPStatus   int
	ProviderCode string
	RetryAfter   time.Duration
	RequestID    string
	Message      string
}

/*
Error renders the failure for logs, without the key or prompt.
*/
func (e *LLMError) Error() string {
	return "llm: " + string(e.Kind) + ": " + e.Message
}

/*
Retryable reports whether the retry layer may attempt the call again:
throttling, overload, server faults, timeouts and network blips. Auth,
quota, permission, bad requests and cancellations are final.
*/
func (e *LLMError) Retryable() bool {
	switch e.Kind {
	case LLMErrorRateLimited, LLMErrorOverloaded, LLMErrorServer, LLMErrorTimeout, LLMErrorNetwork:
		return true
	case LLMErrorAuth, LLMErrorPermission, LLMErrorQuota, LLMErrorInvalidRequest,
		LLMErrorModelNotFound, LLMErrorContentFiltered, LLMErrorBlockedHost, LLMErrorCanceled:
		return false
	}
	return false
}

/*
ErrLLMNotSupported marks a provider that has no model list endpoint;
callers fall back to Probe.
*/
var ErrLLMNotSupported = errors.New("llm: operation not supported by this provider")

/*
ErrSchemaUnsupported marks a request with a response schema the model
or driver cannot enforce. Callers treat the model as unfit for typed
answers; no driver ever falls back to asking for JSON in the prompt.
*/
var ErrSchemaUnsupported = errors.New("llm: response schema not supported by this model")
