/*
|--------------------------------------------------------------------------
| Rate Limit Headers
|--------------------------------------------------------------------------
|
| Clients pace themselves when every limited response says where they
| stand. The header names and values follow Laravel's throttle middleware,
| which most HTTP clients and SDKs already understand.
|
*/

package ratelimit

import (
	"net/http"
	"strconv"
	"time"
)

/*
WriteHeaders sets X-RateLimit-Limit and X-RateLimit-Remaining on the
header. A refused result also gets Retry-After in whole seconds, rounded
up, and X-RateLimit-Reset as the Unix time the window resets, counted
from now.
*/
func (r Result) WriteHeaders(h http.Header, now time.Time) {
	h.Set("X-RateLimit-Limit", strconv.Itoa(r.Limit))
	h.Set("X-RateLimit-Remaining", strconv.Itoa(r.Remaining))

	if r.Allowed {
		return
	}

	seconds := max(int64((r.RetryAfter+time.Second-1)/time.Second), 1)
	h.Set("Retry-After", strconv.FormatInt(seconds, 10))
	h.Set("X-RateLimit-Reset", strconv.FormatInt(now.Unix()+seconds, 10))
}
