/*
|--------------------------------------------------------------------------
| Gmail Errors
|--------------------------------------------------------------------------
|
| Google answers failures with a status and a short reason. Rate limits
| arrive as 403 as well as 429, so the reason decides. A token refused
| after being refreshed means the grant is gone, while a mailbox-level
| refusal is only out of reach for now, so it is retried before anyone is
| asked to reconnect.
|
*/

package gmail

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/mailbox"
)

/*
statusError is a failure the callers look at by status.
*/
type statusError struct {
	status int
	reason string
}

func (e *statusError) Error() string {
	return fmt.Sprintf("gmail: status %d: %s", e.status, e.reason)
}

/*
reasonOf reads Google's reason out of an error body.
*/
func reasonOf(body []byte) string {
	var answer struct {
		Error struct {
			Errors []struct {
				Reason string `json:"reason"`
			} `json:"errors"`
			Status string `json:"status"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &answer) != nil {
		return ""
	}

	if len(answer.Error.Errors) > 0 {
		return answer.Error.Errors[0].Reason
	}

	return answer.Error.Status
}

/*
check maps a read's failure.
*/
func check(resp *httpclient.Response) error {
	if resp.Status >= http.StatusOK && resp.Status < http.StatusMultipleChoices {
		return nil
	}

	reason := reasonOf(resp.Body)

	switch {
	case resp.Status == http.StatusUnauthorized:
		return fmt.Errorf("%w: gmail status 401", mailbox.ErrGrantRevoked)
	case rateLimited(resp.Status, reason):
		return &mailbox.RateLimitedError{RetryAfter: retryAfter(resp)}
	case reason == "failedPrecondition", resp.Status == http.StatusForbidden:
		return fmt.Errorf("%w: gmail %s", mailbox.ErrAccountGone, reason)
	default:
		return &statusError{status: resp.Status, reason: reason}
	}
}

/*
checkSend maps a send's failure, where a refusal is permanent rather than
a sign the mailbox went away.
*/
func checkSend(resp *httpclient.Response) error {
	reason := reasonOf(resp.Body)
	refused := resp.Status == http.StatusForbidden || resp.Status == http.StatusBadRequest ||
		reason == "failedPrecondition"

	if refused && !rateLimited(resp.Status, reason) {
		return fmt.Errorf("%w: gmail %s", mailbox.ErrSendRefused, reason)
	}

	return check(resp)
}

func rateLimited(status int, reason string) bool {
	return status == http.StatusTooManyRequests ||
		slices.Contains([]string{"rateLimitExceeded", "userRateLimitExceeded"}, reason)
}

func retryAfter(resp *httpclient.Response) time.Duration {
	seconds, err := strconv.Atoi(resp.Header.Get("Retry-After"))
	if err != nil {
		return 0
	}

	return time.Duration(seconds) * time.Second
}
