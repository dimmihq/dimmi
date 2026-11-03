/*
|--------------------------------------------------------------------------
| Activate Auth Factor
|--------------------------------------------------------------------------
|
| A pending factor counts once the user proves it works: the first code
| from their authenticator. The code is the proof, so no fresh password
| is needed; starting the enrollment already asked for one. Five wrong
| codes throw the pending factor away and the user starts again.
|
| The first factor that protects the account turns two-factor
| authentication on: the account is marked, a set of recovery codes is
| made and shown once, the owner is mailed, and the browser's session
| gets a new id, since what it is worth just changed.
|
*/

package factors

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/ratelimit"
	"github.com/dimmihq/dimmi/framework/session"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/auth/factors"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/config"
)

/*
maxActivationMisses is how many wrong codes a pending factor takes
before it is thrown away.
*/
const maxActivationMisses = 5

/*
ActivateAuthFactorInput is the user, the pending factor, the code from
it and the client.
*/
type ActivateAuthFactorInput struct {
	User     models.User
	FactorID ids.UUID
	Code     string
	Client   appauth.Client
}

/*
Activated is the factor just activated, and the recovery codes made
when it turned two-factor authentication on, nil otherwise.
*/
type Activated struct {
	Factor        models.UserAuthFactor
	RecoveryCodes *NewAuthFactor
}

/*
ActivateAuthFactor turns a pending factor active with a code from it.

Rules:
  - At most ten tries a minute per user.
  - Only the user's own pending factor of an enabled kind, started
    within the pending lifetime, is found; an active one cannot be
    activated again.
  - A wrong code is counted, and the fifth deletes the pending factor.
  - The factor becomes preferred when the user has no preferred one.
  - The first factor that turns two-factor authentication on marks the
    account, makes recovery codes, mails the owner and rotates the
    browser session.

Errors: [exceptions.ErrNotFound], [exceptions.ErrFactorAlreadyActive],
[exceptions.ErrInvalidCode], [exceptions.ErrTooManyAttempts], or a
wrapped error.
*/
type ActivateAuthFactor struct {
	DB          *database.DB
	Clock       clock.Clock
	Mail        *mail.Manager
	Limiter     *ratelimit.Limiter
	Links       appauth.Links
	Factors     *factors.Registry
	Config      config.FactorsConfig
	SecurityLog *appauth.SecurityLog
}

/*
Handle checks the code and activates the factor.
*/
func (a *ActivateAuthFactor) Handle(ctx context.Context, in ActivateAuthFactorInput) (Activated, error) {
	err := appauth.Throttle(ctx, a.Limiter, appauth.Rate{
		Key: "factors:activate:" + in.User.ID.String(), Limit: 10, Window: time.Minute,
	})
	if err != nil {
		return Activated{}, err
	}

	var (
		out   Activated
		wrong bool
	)

	err = a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		user, err := models.New(a.DB).LockUser(ctx, in.User.ID)
		if err != nil {
			return fmt.Errorf("lock user to activate factor: %w", err)
		}

		in.User = user

		f, driver, err := a.pending(ctx, in)
		if err != nil {
			return err
		}

		err = driver.Verify(ctx, &f, in.Code)
		if errors.Is(err, factors.ErrInvalidCode) || errors.Is(err, factors.ErrCodeReused) {
			wrong = true

			return nil
		}

		if err != nil {
			return fmt.Errorf("check activation code: %w", err)
		}

		out, err = a.activate(ctx, tx, in, f)

		return err
	})
	if err != nil {
		return Activated{}, err
	}

	if wrong {
		return Activated{}, a.miss(ctx, in)
	}

	return out, nil
}

/*
pending locks and returns the user's live pending factor with its
driver.
*/
func (a *ActivateAuthFactor) pending(ctx context.Context, in ActivateAuthFactorInput) (
	models.UserAuthFactor, factors.Factor, error,
) {
	f, err := models.New(a.DB).LockUserAuthFactor(ctx, models.LockUserAuthFactorParams{
		ID: in.FactorID, UserID: in.User.ID,
	})
	if errors.Is(err, database.ErrNotFound) {
		return f, nil, exceptions.ErrNotFound
	}

	if err != nil {
		return f, nil, fmt.Errorf("lock factor to activate: %w", err)
	}

	if f.Status == string(enums.AuthFactorStatusActive) {
		return f, nil, exceptions.ErrFactorAlreadyActive
	}

	driver, ok := a.Factors.Find(f.Type)
	if !ok || !f.CreatedAt.After(a.Clock.Now().Add(-a.Config.PendingTTL)) {
		return f, nil, exceptions.ErrNotFound
	}

	return f, driver, nil
}

/*
activate stores the factor as active and, when it is the first to
protect the account, turns two-factor authentication on.
*/
func (a *ActivateAuthFactor) activate(
	ctx context.Context, tx *database.Tx, in ActivateAuthFactorInput, f models.UserAuthFactor,
) (Activated, error) {
	q := models.New(a.DB)
	now := a.Clock.Now()

	held, err := q.ListActiveAuthFactors(ctx, in.User.ID)
	if err != nil {
		return Activated{}, fmt.Errorf("list factors to activate one: %w", err)
	}

	f, err = q.ActivateAuthFactor(ctx, models.ActivateAuthFactorParams{
		ID: f.ID, LastCounter: f.LastCounter, IsPreferred: !anyPreferred(held), Now: now,
	})
	if err != nil {
		return Activated{}, fmt.Errorf("activate factor: %w", err)
	}

	err = a.SecurityLog.Record(ctx, appauth.Entry{
		UserID: in.User.ID, Type: appauth.EventAuthFactorAdded, Client: in.Client, Metadata: factorEvent(f),
	})
	if err != nil {
		return Activated{}, err
	}

	out := Activated{Factor: f}
	if !a.Factors.EnablesTwoFactor(f.Type) || in.User.TwoFactorEnabled() {
		return out, nil
	}

	codes, err := a.enable(ctx, tx, in)
	out.RecoveryCodes = &codes

	return out, err
}

/*
enable turns two-factor authentication on for the user: marks the
account, makes recovery codes, logs and mails it and rotates the
browser session.
*/
func (a *ActivateAuthFactor) enable(ctx context.Context, tx *database.Tx, in ActivateAuthFactorInput) (
	NewAuthFactor, error,
) {
	now := a.Clock.Now()
	q := models.New(a.DB)

	err := q.UpdateUserTwoFactorConfirmedAt(ctx, models.UpdateUserTwoFactorConfirmedAtParams{
		ID: in.User.ID, TwoFactorConfirmedAt: &now, Now: now,
	})
	if err != nil {
		return NewAuthFactor{}, fmt.Errorf("turn two-factor authentication on: %w", err)
	}

	codes, err := a.recoveryCodes(ctx, q, in.User)
	if err != nil {
		return NewAuthFactor{}, err
	}

	err = a.SecurityLog.Record(ctx, appauth.Entry{
		UserID: in.User.ID, Type: appauth.EventTwoFactorEnabled, Client: in.Client,
	})
	if err != nil {
		return NewAuthFactor{}, err
	}

	err = appmail.ToUser(a.Mail, in.User).QueueTx(ctx, tx, appmail.TwoFactorEnabledMail{
		Name: in.User.Name, SecurityURL: a.Links.Security(),
	})
	if err != nil {
		return NewAuthFactor{}, fmt.Errorf("queue two-factor enabled mail: %w", err)
	}

	rotateSession(ctx, in.User.ID)

	return codes, nil
}

/*
rotateSession gives the browser session of the user a new id, keeping
how it signed in. A request from an app has no such session.
*/
func rotateSession(ctx context.Context, userID ids.UUID) {
	s, ok := session.From(ctx)
	if !ok {
		return
	}

	if current, signedIn := s.UserID(); signedIn && current == userID {
		s.Regenerate()
	}
}

/*
recoveryCodes makes the user's set of recovery codes, or none while
recovery codes are disabled.
*/
func (a *ActivateAuthFactor) recoveryCodes(ctx context.Context, q *models.Queries, user models.User) (
	NewAuthFactor, error,
) {
	driver, ok := a.Factors.Find(factors.RecoveryCodeType)
	if !ok {
		return NewAuthFactor{}, nil
	}

	return writer{q: q, now: a.Clock.Now()}.replaceRecoveryCodes(ctx, driver, user)
}

/*
miss counts a wrong activation code and deletes the pending factor at
the last one allowed.
*/
func (a *ActivateAuthFactor) miss(ctx context.Context, in ActivateAuthFactorInput) error {
	within, err := appauth.Within(ctx, a.Limiter, appauth.Rate{
		Key:    "factors:activate:misses:" + in.FactorID.String(),
		Limit:  maxActivationMisses - 1,
		Window: a.Config.PendingTTL,
	})
	if err != nil {
		return err
	}

	if within {
		return exceptions.ErrInvalidCode
	}

	err = models.New(a.DB).DeleteAuthFactor(ctx, models.DeleteAuthFactorParams{ID: in.FactorID, UserID: in.User.ID})
	if err != nil {
		return fmt.Errorf("delete pending factor after wrong codes: %w", err)
	}

	return exceptions.ErrInvalidCode
}

/*
anyPreferred reports whether one of the factors is preferred.
*/
func anyPreferred(held []models.UserAuthFactor) bool {
	for _, f := range held {
		if f.IsPreferred {
			return true
		}
	}

	return false
}
