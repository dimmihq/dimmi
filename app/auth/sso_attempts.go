/*
|--------------------------------------------------------------------------
| Single Sign-On Attempts
|--------------------------------------------------------------------------
|
| A sign-in through a company's identity provider leaves the site and
| comes back on a post from another origin, so neither the session nor
| the usual request checks can tie the answer to the browser that asked.
| The attempt does: it lives in the cache for ten minutes under a random
| relay state the identity provider echoes back, and names a second
| random value held in a cookie of that browser. Both must match, and an
| attempt is claimed once. An assertion is accepted once per connection
| for as long as it stays valid. Every check fails closed: with the cache
| down nobody signs in this way.
|
*/

package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/cache"
	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/support/str"
)

/*
SSOAttemptLifetime is how long a single sign-on attempt waits for the
identity provider's answer.
*/
const SSOAttemptLifetime = 10 * time.Minute

/*
The intents of a single sign-on attempt: signing in, or an owner trying
a connection out without starting a session.
*/
const (
	SSOIntentLogin = "login"
	SSOIntentTest  = "test"
)

/*
ErrSSOStoreDown means the attempt store could not be reached, so no
sign-in through a connection may go on.
*/
var ErrSSOStoreDown = errors.New("sso: attempt store unavailable")

/*
SSOAttempt is one sign-in waiting on its identity provider: the
connection, the id of the request sent, why it was started, who started
it when someone was signed in, and where to land after.
*/
type SSOAttempt struct {
	ConnectionID ids.UUID `json:"connection_id"`
	RequestID    string   `json:"request_id"`
	Intent       string   `json:"intent"`
	UserID       ids.UUID `json:"user_id"`
	ReturnTo     string   `json:"return_to"`
	Browser      string   `json:"browser"`
}

/*
SSOAttempts keeps single sign-on attempts and the assertions already
used.
*/
type SSOAttempts struct {
	Cache *cache.Store
}

/*
Relay returns a fresh relay state and browser value for an attempt.
*/
func (s SSOAttempts) Relay() (relay, browser string) {
	return str.Random(32), str.Random(32)
}

/*
Remember stores the attempt under the relay state.
*/
func (s SSOAttempts) Remember(ctx context.Context, relay string, attempt SSOAttempt) error {
	raw, err := json.Marshal(attempt)
	if err != nil {
		return fmt.Errorf("encode sso attempt: %w", err)
	}

	if err := s.Cache.Set(ctx, attemptKey(relay), raw, SSOAttemptLifetime); err != nil {
		return fmt.Errorf("%w: %w", ErrSSOStoreDown, err)
	}

	return nil
}

/*
Claim returns the attempt the relay state names when the browser value
matches, and makes sure nobody claims it again. It reports false for an
unknown, expired, foreign or already claimed attempt.
*/
func (s SSOAttempts) Claim(ctx context.Context, relay, browser string) (SSOAttempt, bool, error) {
	if relay == "" || browser == "" {
		return SSOAttempt{}, false, nil
	}

	raw, err := s.Cache.Get(ctx, attemptKey(relay))
	if errors.Is(err, cache.ErrMiss) {
		return SSOAttempt{}, false, nil
	}

	if err != nil {
		return SSOAttempt{}, false, fmt.Errorf("%w: %w", ErrSSOStoreDown, err)
	}

	attempt, ok := matching(raw, browser)
	if !ok {
		return SSOAttempt{}, false, nil
	}

	first, err := s.Cache.Add(ctx, attemptKey(relay)+":claimed", []byte{1}, SSOAttemptLifetime)
	if err != nil {
		return SSOAttempt{}, false, fmt.Errorf("%w: %w", ErrSSOStoreDown, err)
	}

	return attempt, first, nil
}

/*
FirstUse reports whether the assertion is used for the first time
through the connection, remembering it until it would have expired.
*/
func (s SSOAttempts) FirstUse(
	ctx context.Context, connection ids.UUID, assertion string, until time.Duration,
) (bool, error) {
	if until <= 0 {
		until = time.Minute
	}

	first, err := s.Cache.Add(ctx, "sso:assertion:"+connection.String()+":"+assertion, []byte{1}, until)
	if err != nil {
		return false, fmt.Errorf("%w: %w", ErrSSOStoreDown, err)
	}

	return first, nil
}

/*
matching reads the stored attempt and reports whether the browser value
is the one it was started with.
*/
func matching(raw []byte, browser string) (SSOAttempt, bool) {
	var attempt SSOAttempt
	if err := json.Unmarshal(raw, &attempt); err != nil {
		return SSOAttempt{}, false
	}

	return attempt, hashing.Equal([]byte(attempt.Browser), []byte(browser))
}

/*
attemptKey is the cache key of the attempt the relay state names.
*/
func attemptKey(relay string) string {
	return "sso:attempt:" + relay
}
