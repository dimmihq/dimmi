/*
|--------------------------------------------------------------------------
| Sign-In Lockout
|--------------------------------------------------------------------------
|
| Password guessing is pushed back on three fronts, all counted in
| Valkey so every server shares them:
|
|   - one address guessing one account: a few attempts a minute, and a
|     lock that grows with each run of five failures, from 30 seconds to
|     an hour, cleared by a successful sign-in;
|   - one account guessed from anywhere: after many failures in an hour
|     the account is locked for strangers for 15 minutes, while a browser
|     that signed in as this user before keeps getting in, so an
|     attacker can never lock the real user out;
|   - one address guessing many accounts: blocked for an hour.
|
| A lock answers before any password is hashed, the same way for every
| address, and a correct password does not get past it. Locks end by
| the application's clock, which tests can move; Valkey only forgets
| them later. Losing Valkey forgets every count, which is acceptable.
|
*/

package auth

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/dimmihq/dimmi/framework/cache"
	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/ratelimit"

	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/config"
)

/*
The windows the lockout counts in and the locks it hands out.
*/
const (
	attemptWindow   = time.Minute
	failureWindow   = time.Hour
	streakWindow    = 24 * time.Hour
	emailLock       = 15 * time.Minute
	sprayBlock      = time.Hour
	streakStep      = 5
	unlimitedStreak = 1 << 20
)

/*
Attempt is one sign-in try: from an address, at an email, by a browser
that has or has not signed in as this account before.
*/
type Attempt struct {
	IP          string
	Email       string
	KnownDevice bool
}

/*
Lockout counts failed sign-ins and refuses the ones it locked out.
*/
type Lockout struct {
	Limiter *ratelimit.Limiter
	Cache   *cache.Store
	Clock   clock.Clock
	Config  config.LockoutConfig
}

/*
Check returns exceptions.ErrTooManyAttempts, with how long to wait,
when the attempt is locked out, and counts it otherwise. A locked
attempt is not counted.
*/
func (l *Lockout) Check(ctx context.Context, a Attempt) error {
	locks := []string{l.sprayKey(a), l.streakKey(a)}
	if !a.KnownDevice {
		locks = append(locks, l.emailKey(a))
	}

	for _, key := range locks {
		wait, err := l.lockedFor(ctx, key)
		if err != nil {
			return Unavailable(ctx, err)
		}

		if wait > 0 {
			return tooManyAttempts(wait)
		}
	}

	res, err := l.Limiter.Attempt(ctx, "login:attempts:"+a.IP+":"+a.Email, l.Config.PerIPEmail, attemptWindow)
	if err != nil {
		return Unavailable(ctx, err)
	}

	if !res.Allowed {
		return tooManyAttempts(res.RetryAfter)
	}

	return nil
}

/*
Failed counts a failed attempt on every front and locks what crossed
its line. It reports whether this failure locked anything.
*/
func (l *Lockout) Failed(ctx context.Context, a Attempt) (bool, error) {
	locked, err := l.countFailure(ctx, a)
	if err != nil {
		return false, Unavailable(ctx, err)
	}

	return locked, nil
}

/*
countFailure counts the failure and locks what crossed its line.
*/
func (l *Lockout) countFailure(ctx context.Context, a Attempt) (bool, error) {
	streak, streakErr := l.count(ctx, "login:streak:"+a.IP+":"+a.Email, unlimitedStreak, streakWindow)
	byEmail, emailErr := l.count(ctx, "login:failures:email:"+a.Email, l.Config.PerEmailSoftLock, failureWindow)
	byIP, ipErr := l.count(ctx, "login:failures:ip:"+a.IP, l.Config.PerIPSpray, failureWindow)

	if err := errors.Join(streakErr, emailErr, ipErr); err != nil {
		return false, err
	}

	locks := []struct {
		crossed bool
		key     string
		wait    time.Duration
	}{
		{streakLock(streak) > 0, l.streakKey(a), streakLock(streak)},
		{byEmail >= l.Config.PerEmailSoftLock, l.emailKey(a), emailLock},
		{byIP >= l.Config.PerIPSpray, l.sprayKey(a), sprayBlock},
	}

	locked, errs := false, []error{}

	for _, lock := range locks {
		if lock.crossed {
			errs = append(errs, l.lock(ctx, lock.key, lock.wait))
			locked = true
		}
	}

	return locked, errors.Join(errs...)
}

/*
Succeeded ends the run of failures of the address at the email. The
hourly counts stay, so a success does not reset a spray in progress.
*/
func (l *Lockout) Succeeded(ctx context.Context, a Attempt) error {
	if err := l.Limiter.Clear(ctx, "login:streak:"+a.IP+":"+a.Email); err != nil {
		return Unavailable(ctx, err)
	}

	return nil
}

/*
streakLock returns how long a run of consecutive failures locks the
address at the email: 30 seconds at five, five minutes at ten, half an
hour at fifteen, and an hour for every failure from twenty on.
*/
func streakLock(streak int) time.Duration {
	if streak >= 4*streakStep {
		return time.Hour
	}

	return map[int]time.Duration{
		streakStep:     30 * time.Second,
		2 * streakStep: 5 * time.Minute,
		3 * streakStep: 30 * time.Minute,
	}[streak]
}

/*
count adds a failure to the counter and returns how many it holds now,
up to the limit.
*/
func (l *Lockout) count(ctx context.Context, key string, limit int, window time.Duration) (int, error) {
	res, err := l.Limiter.Hit(ctx, key, limit, window)

	return res.Limit - res.Remaining, err
}

/*
lock locks the key until the wait is over by the application's clock.
*/
func (l *Lockout) lock(ctx context.Context, key string, wait time.Duration) error {
	until := l.Clock.Now().Add(wait).Unix()

	return l.Cache.Set(ctx, key, []byte(strconv.FormatInt(until, 10)), wait+time.Minute)
}

/*
lockedFor returns how long the key stays locked, or zero.
*/
func (l *Lockout) lockedFor(ctx context.Context, key string) (time.Duration, error) {
	stored, err := l.Cache.Get(ctx, key)
	if errors.Is(err, cache.ErrMiss) {
		return 0, nil
	}

	if err != nil {
		return 0, fmt.Errorf("read sign-in lock: %w", err)
	}

	until, err := strconv.ParseInt(string(stored), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("read sign-in lock %q: %w", key, err)
	}

	return max(time.Unix(until, 0).Sub(l.Clock.Now()), 0), nil
}

func (l *Lockout) streakKey(a Attempt) string {
	return "login:lock:" + a.IP + ":" + a.Email
}

func (l *Lockout) emailKey(a Attempt) string {
	return "login:lock:email:" + a.Email
}

func (l *Lockout) sprayKey(a Attempt) string {
	return "login:lock:ip:" + a.IP
}

/*
tooManyAttempts is the locked answer, telling the client to wait at
least a second.
*/
func tooManyAttempts(wait time.Duration) error {
	return exceptions.ErrTooManyAttempts.With("retry_after", max(int((wait+time.Second-1)/time.Second), 1))
}
