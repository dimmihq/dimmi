/*
|--------------------------------------------------------------------------
| Graph Errors
|--------------------------------------------------------------------------
|
| Graph answers failures with a status and a code. A token refused after
| being refreshed means the grant is gone. A mailbox out of license, or a
| shared mailbox no longer shared, is only out of reach for now. A delta
| link Graph forgot is recovered by listing recent mail.
|
*/

package graph

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
statusError is any other failure, which is worth a retry.
*/
type statusError struct {
	status int
	code   string
}

func (e *statusError) Error() string {
	return fmt.Sprintf("graph: status %d: %s", e.status, e.code)
}

func codeOf(body []byte) string {
	var answer struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &answer) != nil {
		return ""
	}

	return answer.Error.Code
}

/*
check maps a call's failure.
*/
func check(resp *httpclient.Response) error {
	if resp.Status >= http.StatusOK && resp.Status < http.StatusMultipleChoices {
		return nil
	}

	code := codeOf(resp.Body)

	switch {
	case resp.Status == http.StatusUnauthorized:
		return fmt.Errorf("%w: graph status 401", mailbox.ErrGrantRevoked)
	case resp.Status == http.StatusTooManyRequests:
		return &mailbox.RateLimitedError{RetryAfter: retryAfter(resp)}
	case resp.Status == http.StatusGone || code == "SyncStateNotFound" || code == "ResyncRequired":
		return mailbox.ErrCursorExpired
	case resp.Status == http.StatusForbidden || resp.Status == http.StatusNotFound:
		return fmt.Errorf("%w: graph %s", mailbox.ErrAccountGone, code)
	default:
		return &statusError{status: resp.Status, code: code}
	}
}

/*
checkSend maps a send's failure, where some refusals never change.
*/
func checkSend(resp *httpclient.Response) error {
	refused := []string{"ErrorMessageSizeExceeded", "ErrorSendAsDenied", "ErrorInvalidRecipients"}
	if slices.Contains(refused, codeOf(resp.Body)) {
		return fmt.Errorf("%w: graph %s", mailbox.ErrSendRefused, codeOf(resp.Body))
	}

	return check(resp)
}

func retryAfter(resp *httpclient.Response) time.Duration {
	seconds, err := strconv.Atoi(resp.Header.Get("Retry-After"))
	if err != nil {
		return 0
	}

	return time.Duration(seconds) * time.Second
}
