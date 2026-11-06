/*
|--------------------------------------------------------------------------
| Anthropic Error Mapping
|--------------------------------------------------------------------------
|
| Anthropic's dialect becomes the shared kinds here, so retries and
| key health never read a provider message.
|
*/

package anthropic

import (
	"context"
	"errors"
	"time"

	anth "github.com/anthropics/anthropic-sdk-go"

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

	if apiErr, ok := errors.AsType[*anth.Error](err); ok {
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
mapAPIError reads the typed SDK error.
*/
func mapAPIError(err *anth.Error) error {
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
	case fhttp.StatusRequestTimeout:
		kind = contracts.LLMErrorTimeout
	case fhttp.StatusInternalServerError, fhttp.StatusBadGateway, fhttp.StatusGatewayTimeout:
		kind = contracts.LLMErrorServer
	case 529:
		kind = contracts.LLMErrorOverloaded
	}

	return &contracts.LLMError{
		Kind:       kind,
		HTTPStatus: err.StatusCode,
		RetryAfter: retryAfter(err),
		Message:    err.Error(),
	}
}

/*
retryAfter reads the provider's retry hint where present.
*/
func retryAfter(err *anth.Error) time.Duration {
	if err.Response != nil {
		if v := err.Response.Header.Get("Retry-After"); v != "" {
			if secs, parseErr := time.ParseDuration(v + "s"); parseErr == nil {
				return secs
			}
		}
	}
	return 0
}
