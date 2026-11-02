/*
|--------------------------------------------------------------------------
| Push Service Answers
|--------------------------------------------------------------------------
|
| A push service that refuses a message says why with its status code.
| The status is kept whole, so the application can act on the ones that
| matter to it: a subscription that is gone must be deleted, a payload
| too large can be sent again smaller, a rate limit says when to come
| back. Client errors other than a timeout or a rate limit are permanent:
| sending the same message again cannot succeed.
|
*/

package webpush

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/notifications"
)

/*
Web Push errors.

ErrGone means the subscription expired or was withdrawn: the push
service answered 404 or 410, and the subscription should be deleted.
ErrInvalidConfig refuses missing or malformed VAPID settings.
ErrInvalidSubscription refuses a subscription whose endpoint is not an
https URL or whose keys are not valid.
*/
var (
	ErrGone                = errors.New("webpush: subscription gone")
	ErrInvalidConfig       = errors.New("webpush: invalid config")
	ErrInvalidSubscription = errors.New("webpush: invalid subscription")
)

/*
StatusError is a push service's refusal of a message. Endpoint is the
subscription's endpoint, Status the HTTP status, Body the start of the
response body, and RetryAfter how long a rate-limited service asked to
wait, zero when it did not say.

It matches ErrGone for 404 and 410, and notifications.ErrPermanent for
any client error other than 408 and 429.
*/
type StatusError struct {
	Endpoint   string
	Status     int
	Body       string
	RetryAfter time.Duration
}

/*
Error names the push service's host and status. The endpoint itself is
left out, since its path is a secret of the subscription.
*/
func (e *StatusError) Error() string {
	host := "push service"
	if u, err := url.Parse(e.Endpoint); err == nil {
		host = u.Host
	}

	return fmt.Sprintf("webpush: %s answered %d %s", host, e.Status, e.Body)
}

/*
Is reports whether the refusal matches ErrGone or
notifications.ErrPermanent.
*/
func (e *StatusError) Is(target error) bool {
	switch target {
	case ErrGone:
		return e.Status == http.StatusNotFound || e.Status == http.StatusGone
	case notifications.ErrPermanent:
		return httpclient.Permanent(e.Status)
	default:
		return false
	}
}
