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

	"github.com/dimmihq/dimmi/app/contracts"
)

/*
MapError folds an SDK error into a typed LLMError.
*/
func MapError(err error) error {
	if err == nil {
		return nil
	}

	var apiErr *oai.Error
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
mapAPIError reads the typed SDK error for status, code and headers.
*/
func mapAPIError(err *oai.Error) error {
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
		if code := err.Code; code == "insufficient_quota" || code == "credit_balance_exhausted" {
			kind = contracts.LLMErrorQuota
		}
	case 408:
		kind = contracts.LLMErrorTimeout
	case 500, 502, 504:
		kind = contracts.LLMErrorServer
	case 503:
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
