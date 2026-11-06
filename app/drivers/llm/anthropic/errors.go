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

	"github.com/dimmihq/dimmi/app/contracts"
)

/*
MapError folds an SDK error into a typed LLMError.
*/
func MapError(err error) error {
	if err == nil {
		return nil
	}

	var apiErr *anth.Error
	if errors.As(err, &apiErr) {
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
	case 401:
		kind = contracts.LLMErrorAuth
	case 403:
		kind = contracts.LLMErrorPermission
	case 404:
		kind = contracts.LLMErrorModelNotFound
	case 400, 422:
		kind = contracts.LLMErrorInvalidRequest
	case 402:
		kind = contracts.LLMErrorQuota
	case 429:
		kind = contracts.LLMErrorRateLimited
	case 408:
		kind = contracts.LLMErrorTimeout
	case 500, 502, 504:
		kind = contracts.LLMErrorServer
	case 529:
		kind = contracts.LLMErrorOverloaded
	}

	return &contracts.LLMError{
		Kind:         kind,
		HTTPStatus:   err.StatusCode,
		RetryAfter:   retryAfter(err),
		Message:      err.Error(),
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
