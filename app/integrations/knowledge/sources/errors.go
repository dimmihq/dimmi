/*
|--------------------------------------------------------------------------
| Driver Errors
|--------------------------------------------------------------------------
|
| Drivers answer in a few shared errors so the engine treats every
| source the same: a refused key stops the sync and tells the admins, a
| missing document is removed, a busy source is waited for, and an
| excluded document is kept out with its reason shown.
|
*/

package sources

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/dimmihq/dimmi/framework/httpclient"
)

/*
The errors drivers share. ErrUnauthorized means the source refused the
credentials, ErrNotFound that a document or the source itself is gone,
and ErrUnavailable that the source failed in a way worth retrying.
*/
var (
	ErrUnauthorized = errors.New("sources: the source refused the credentials")
	ErrNotFound     = errors.New("sources: not found at the source")
	ErrUnavailable  = errors.New("sources: the source is unavailable")
)

/*
ConfigError is a setting the driver cannot work with; Field names it
when one is to blame.
*/
type ConfigError struct {
	Field   string
	Message string
}

func (e *ConfigError) Error() string {
	return e.Message
}

/*
RateLimitError asks to wait before the source is called again.
*/
type RateLimitError struct {
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("sources: rate limited, retry after %s", e.RetryAfter)
}

/*
WaitError says the source is still working, such as a crawl job not
finished yet: the same call is made again after the wait, without it
counting as a failure.
*/
type WaitError struct {
	After time.Duration
}

func (e *WaitError) Error() string {
	return fmt.Sprintf("sources: not ready, ask again in %s", e.After)
}

/*
ExcludedError keeps a document out of the index. Reason is one of
robots, pattern, js_rendered, duplicate, too_large, unsupported_type,
empty and draft.
*/
type ExcludedError struct {
	Reason string
}

func (e *ExcludedError) Error() string {
	return "sources: excluded (" + e.Reason + ")"
}

/*
Excluded returns an ExcludedError for the reason.
*/
func Excluded(reason string) error {
	return &ExcludedError{Reason: reason}
}

/*
maxRetryAfter caps how long a source may ask to be left alone.
*/
const maxRetryAfter = time.Hour

/*
FromResponse maps an answer of a source's HTTP API to the shared
errors: nil for success, ErrUnauthorized for 401 and 403, ErrNotFound
for 404 and 410, a RateLimitError for 429, ErrUnavailable for server
errors and a plain error naming the status otherwise.
*/
func FromResponse(resp *httpclient.Response, what string) error {
	switch {
	case resp.Status >= 200 && resp.Status < 300:
		return nil
	case resp.Status == 401 || resp.Status == 403:
		return fmt.Errorf("%s: %w", what, ErrUnauthorized)
	case resp.Status == 404 || resp.Status == 410:
		return fmt.Errorf("%s: %w", what, ErrNotFound)
	case resp.Status == 429:
		return &RateLimitError{RetryAfter: RetryAfter(resp.Header.Get("Retry-After"))}
	case resp.Status >= 500:
		return fmt.Errorf("%s: status %d: %w", what, resp.Status, ErrUnavailable)
	}

	return fmt.Errorf("%s: status %d", what, resp.Status)
}

/*
RetryAfter reads a Retry-After header in seconds, a minute when it is
missing or unreadable, never more than an hour.
*/
func RetryAfter(header string) time.Duration {
	seconds, err := strconv.Atoi(header)
	if err != nil || seconds <= 0 {
		return time.Minute
	}

	return min(time.Duration(seconds)*time.Second, maxRetryAfter)
}
