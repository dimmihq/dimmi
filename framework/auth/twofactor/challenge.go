/*
|--------------------------------------------------------------------------
| Challenges
|--------------------------------------------------------------------------
|
| Between a correct password and a correct second factor the user is
| neither in nor out. A challenge remembers that state for a few minutes:
| who passed the first step, which factors they may answer with, and any
| state the flow needs to carry on afterwards. The browser holds only a
| random token; the cache keys the challenge by its digest.
|
| Guessing is bounded twice over: a challenge accepts only a few wrong
| answers before it is burned, and it works for one success only, so a
| replayed request after login finds nothing.
|
*/

package twofactor

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/cache"
	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/ids"
)

/*
ErrChallengeExpired means the challenge is unknown, expired, burned by
too many wrong answers or already completed. The user starts over.
*/
var ErrChallengeExpired = errors.New("twofactor: challenge expired")

/*
challengeTokenBytes is the number of random bytes in a challenge token.
*/
const challengeTokenBytes = 32

/*
Challenge is a pending second-factor step.

UserID passed the first step and Methods are the factor types offered.
Data carries the flow's own state, such as where to go after login.
ExpiresAt is set when the challenge begins.
*/
type Challenge struct {
	UserID    ids.UUID          `json:"user_id"`
	Methods   []string          `json:"methods"`
	Data      map[string]string `json:"data,omitempty"`
	ExpiresAt time.Time         `json:"expires_at"`
}

/*
Challenges keeps pending challenges in the cache.
*/
type Challenges struct {
	cache    *cache.Store
	clock    clock.Clock
	ttl      time.Duration
	attempts int64
}

/*
NewChallenges returns challenges that last for the ttl and are burned
after the given number of wrong answers. Zero values fall back to five
minutes and five answers.
*/
func NewChallenges(store *cache.Store, clk clock.Clock, ttl time.Duration, attempts int) *Challenges {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}

	if attempts <= 0 {
		attempts = 5
	}

	return &Challenges{cache: store, clock: clk, ttl: ttl, attempts: int64(attempts)}
}

/*
Begin stores the challenge and returns the token to hand the browser,
with the challenge as stored.
*/
func (c *Challenges) Begin(ctx context.Context, ch Challenge) (string, Challenge, error) {
	raw := make([]byte, challengeTokenBytes)
	_, _ = rand.Read(raw)
	token := base64.RawURLEncoding.EncodeToString(raw)

	ch.ExpiresAt = c.clock.Now().Add(c.ttl)

	encoded, err := json.Marshal(ch)
	if err != nil {
		return "", Challenge{}, fmt.Errorf("encode challenge: %w", err)
	}

	if err := c.cache.Set(ctx, challengeKey(token), encoded, c.ttl); err != nil {
		return "", Challenge{}, fmt.Errorf("store challenge: %w", err)
	}

	return token, ch, nil
}

/*
Find returns the live challenge for the token, or ErrChallengeExpired.
*/
func (c *Challenges) Find(ctx context.Context, token string) (Challenge, error) {
	encoded, err := c.cache.Get(ctx, challengeKey(token))
	if errors.Is(err, cache.ErrMiss) {
		return Challenge{}, ErrChallengeExpired
	}

	if err != nil {
		return Challenge{}, fmt.Errorf("load challenge: %w", err)
	}

	var ch Challenge
	if err := json.Unmarshal(encoded, &ch); err != nil {
		return Challenge{}, fmt.Errorf("decode challenge: %w", err)
	}

	if !c.clock.Now().Before(ch.ExpiresAt) {
		return Challenge{}, ErrChallengeExpired
	}

	return ch, nil
}

/*
Fail counts a wrong answer and returns how many are left. The answer
that uses up the last one burns the challenge and returns
ErrChallengeExpired.
*/
func (c *Challenges) Fail(ctx context.Context, token string) (int, error) {
	key := challengeKey(token)

	failed, err := c.cache.Increment(ctx, key+":failures", 1, c.ttl)
	if err != nil {
		return 0, fmt.Errorf("count challenge failure: %w", err)
	}

	if failed < c.attempts {
		return int(c.attempts - failed), nil
	}

	if err := c.cache.Delete(ctx, key); err != nil {
		return 0, fmt.Errorf("burn challenge: %w", err)
	}

	return 0, ErrChallengeExpired
}

/*
Complete ends the challenge after a correct answer and returns it. Only
the first call for a token succeeds; any other gets ErrChallengeExpired.
*/
func (c *Challenges) Complete(ctx context.Context, token string) (Challenge, error) {
	ch, err := c.Find(ctx, token)
	if err != nil {
		return Challenge{}, err
	}

	key := challengeKey(token)

	first, err := c.cache.Add(ctx, key+":completed", []byte{1}, c.ttl)
	if err != nil {
		return Challenge{}, fmt.Errorf("complete challenge: %w", err)
	}

	if !first {
		return Challenge{}, ErrChallengeExpired
	}

	if err := c.cache.Delete(ctx, key); err != nil {
		return Challenge{}, fmt.Errorf("delete challenge: %w", err)
	}

	return ch, nil
}

func challengeKey(token string) string {
	return "twofactor:challenge:" + hashing.SHA256Hex([]byte(token))
}
