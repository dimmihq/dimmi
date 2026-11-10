/*
|--------------------------------------------------------------------------
| Pacing Email
|--------------------------------------------------------------------------
|
| Two things hold broadcast email back. A newly used address warms up:
| mailbox providers trust a sender that grows slowly, so an address sends
| a few hundred broadcast emails its first day and twice as many each day
| after, until it no longer needs holding back. And the install's mail
| provider grants a quota, such as an SES account's sends per second and
| per day: every broadcast of every workspace shares one slice of it,
| leaving the rest for replies. A held-back email is not a failure; it
| waits in the queue for its turn and the report says why.
|
*/

package broadcasts

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/ratelimit"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/config"
)

/*
Why a recipient waits.
*/
const (
	PacedWarmup   = "warming_up"
	PacedProvider = "provider_limit"
)

/*
PacedError holds a recipient back until a time, for a reason the
report shows.
*/
type PacedError struct {
	Until  time.Time
	Reason string
}

/*
Error says why.
*/
func (e *PacedError) Error() string {
	return "paced: " + e.Reason
}

/*
Pacer decides whether a broadcast email may leave now. Limits are the
install's provider quota, Config the warm-up.
*/
type Pacer struct {
	DB      *database.DB
	Limiter *ratelimit.Limiter
	Clock   clock.Clock
	Limits  config.EmailLimitsConfig
	Config  config.BroadcastsConfig
}

/*
Email returns a *PacedError when the address or the install must wait,
nil when the email may leave now.
*/
func (p *Pacer) Email(ctx context.Context, ws, address ids.UUID) error {
	if err := p.warmup(ctx, ws, address); err != nil {
		return err
	}

	windows := []struct {
		key    string
		quota  int
		window time.Duration
	}{
		{"broadcasts:email:minute", p.Limits.ProviderPerMinute, time.Minute},
		{"broadcasts:email:day", p.Limits.ProviderPerDay, 24 * time.Hour},
	}

	for _, w := range windows {
		if err := p.take(ctx, w.key, p.share(w.quota), w.window); err != nil {
			return err
		}
	}

	return nil
}

/*
warmup holds a young address back once it sent its day's allowance. The
sends of the last day stored in the database are the lasting count; a
counter in Valkey makes the check atomic between parallel senders.
*/
func (p *Pacer) warmup(ctx context.Context, ws, address ids.UUID) error {
	if p.Config.WarmupFirstDay <= 0 {
		return nil
	}

	now := p.Clock.Now()

	var use models.BroadcastAddressUseRow

	err := p.DB.Transaction(withTenant(ctx, ws), func(ctx context.Context, tx *database.Tx) error {
		var err error

		use, err = models.New(tx).BroadcastAddressUse(ctx, models.BroadcastAddressUseParams{
			WorkspaceID: ws, EmailAddressID: &address, Since: now.Add(-24 * time.Hour), Now: now,
		})

		return err
	})
	if err != nil {
		return fmt.Errorf("read address use: %w", err)
	}

	allowance, held := WarmupAllowance(p.Config, now.Sub(use.FirstSent))
	if !held {
		return nil
	}

	if use.Recent >= int64(allowance) {
		return &PacedError{Until: now.Add(time.Hour), Reason: PacedWarmup}
	}

	err = p.take(ctx, "broadcasts:warmup:"+address.String(), allowance, 24*time.Hour)
	if paced, ok := errors.AsType[*PacedError](err); ok {
		paced.Reason = PacedWarmup
	}

	return err
}

/*
WarmupAllowance is how many broadcast emails an address may send in a
day once it has been sending for the age; held is false once it no
longer needs holding back.
*/
func WarmupAllowance(cfg config.BroadcastsConfig, age time.Duration) (int, bool) {
	if cfg.WarmupFirstDay <= 0 {
		return 0, false
	}

	days := max(int(age/(24*time.Hour)), 0)
	allowance := float64(cfg.WarmupFirstDay) * math.Pow(2, float64(min(days, 30)))

	if allowance >= float64(cfg.WarmupUntil) {
		return 0, false
	}

	return int(allowance), true
}

/*
share is the part of a provider quota broadcasts may use; zero means no
quota.
*/
func (p *Pacer) share(quota int) int {
	if quota <= 0 {
		return 0
	}

	percent := p.Limits.BroadcastSharePercent
	if percent <= 0 || percent > 100 {
		percent = 100
	}

	return max(quota*percent/100, 1)
}

/*
take spends one send of the install's quota, or says how long to wait.
*/
func (p *Pacer) take(ctx context.Context, key string, quota int, window time.Duration) error {
	if quota <= 0 || p.Limiter == nil {
		return nil
	}

	res, err := p.Limiter.Attempt(ctx, key, quota, window)
	if err != nil {
		return fmt.Errorf("take send quota: %w", err)
	}

	if res.Allowed {
		return nil
	}

	return &PacedError{Until: p.Clock.Now().Add(max(res.RetryAfter, time.Second)), Reason: PacedProvider}
}
