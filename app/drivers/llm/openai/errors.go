/*
|--------------------------------------------------------------------------
| OpenAI Error Mapping
|--------------------------------------------------------------------------
|
| One place where the provider's dialect becomes the shared kinds, so
| retries, key health and the gate never read a provider message.
|
*/

package openai

import (
	"context"
	"errors"
	"time"

	oai "github.com/openai/openai-go/v3"

	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/contracts"
)

/*
MapError folds an SDK error into a typed LLMError.
*/
func MapError(err error) error {
	if err == nil {
		return nil
	}

	if apiErr, ok := errors.AsType[*oai.Error](err); ok {
		return mapAPIError(apiErr)
	}

	if errors.Is(err, context.Canceled) {
		return &contracts.LLMError{Kind: contracts.LLMErrorCanceled, Message: err.Error()}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return &contracts.LLMError{Kind: contracts.LLMErrorTimeout, Message: err.Error()}
	}

	return &contracts.LLMError{Kind: contracts.LLMErrorNetwork, Message: err.Error()}
}

/*
mapAPIError reads the typed SDK error for status, code and headers.
*/
func mapAPIError(err *oai.Error) error {
	kind := contracts.LLMErrorServer

	switch err.StatusCode {
	case fhttp.StatusUnauthorized:
		kind = contracts.LLMErrorAuth
	case fhttp.StatusForbidden:
		kind = contracts.LLMErrorPermission
	case fhttp.StatusNotFound:
		kind = contracts.LLMErrorModelNotFound
	case fhttp.StatusBadRequest, fhttp.StatusUnprocessableEntity:
		kind = contracts.LLMErrorInvalidRequest
	case fhttp.StatusPaymentRequired:
		kind = contracts.LLMErrorQuota
	case fhttp.StatusTooManyRequests:
		kind = contracts.LLMErrorRateLimited
		if code := err.Code; code == "insufficient_quota" || code == "credit_balance_exhausted" {
			kind = contracts.LLMErrorQuota
		}
	case fhttp.StatusRequestTimeout:
		kind = contracts.LLMErrorTimeout
	case fhttp.StatusInternalServerError, fhttp.StatusBadGateway, fhttp.StatusGatewayTimeout:
		kind = contracts.LLMErrorServer
	case fhttp.StatusServiceUnavailable:
		kind = contracts.LLMErrorOverloaded
	}

	return &contracts.LLMError{
		Kind:         kind,
		HTTPStatus:   err.StatusCode,
		ProviderCode: err.Code,
		RetryAfter:   retryAfter(err),
		Message:      err.Message,
	}
}

/*
retryAfter reads the provider's retry hint where present.
*/
func retryAfter(err *oai.Error) time.Duration {
	if v := err.Response.Header.Get("Retry-After"); v != "" {
		if secs, parseErr := time.ParseDuration(v + "s"); parseErr == nil {
			return secs
		}
	}
	return 0
}
