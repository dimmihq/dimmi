/*
|--------------------------------------------------------------------------
| Retrying Refused Requests
|--------------------------------------------------------------------------
|
| Third-party APIs say the same two things the same way: a status tells
| whether sending the request again can ever work, and a Retry-After
| header tells how long to wait first. Every caller that retries reads
| them here, so they all agree on what is worth another try.
|
*/

package httpclient

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

/*
RetryAfter reads a Retry-After header, given in seconds or as an HTTP
date, as how long to wait from now. A missing, malformed or past value
means the server did not say, and gives zero.
*/
func RetryAfter(header string, now time.Time) time.Duration {
	header = strings.TrimSpace(header)

	if seconds, err := strconv.Atoi(header); err == nil {
		return time.Duration(max(seconds, 0)) * time.Second
	}

	at, err := http.ParseTime(header)
	if err != nil {
		return 0
	}

	return max(at.Sub(now), 0)
}

/*
Permanent reports whether a status refuses the request for good: a
client error other than a timeout or a rate limit, which both pass with
time. Sending the same request again can never succeed.
*/
func Permanent(status int) bool {
	return status >= 400 && status < 500 &&
		status != http.StatusRequestTimeout && status != http.StatusTooManyRequests
}
