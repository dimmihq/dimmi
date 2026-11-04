/*
|--------------------------------------------------------------------------
| Complete Two-Factor Challenge
|--------------------------------------------------------------------------
|
| The second step of signing in: a code from a factor the challenge
| offered, or a passkey's signature over the challenge the server handed
| out with it. The challenge works once and for a few minutes; five
| wrong answers burn it, and each wrong answer also counts toward the
| sign-in lockout, so the second step cannot be guessed at any faster
| than the first. Naming a factor the challenge did not offer is not a
| guess and counts as none.
|
| The factor is locked while its answer is checked, so two tabs sending
| the same code, or a code racing new recovery codes, cannot both win.
| A recovery code is used up and its owner told how many are left.
|
*/

package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/ratelimit"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/auth/factors"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
)

/*
CompleteTwoFactorChallengeInput is the challenge token the browser
holds, the kind of factor answering, the code or the credential a
device signed, and the client.
*/
type CompleteTwoFactorChallengeInput struct {
	Token      string
	Method     string
	Code       string
	Credential json.RawMessage
	Client     appauth.Client
}

/*
challengeData is what a challenge remembers of the first step: how the
user signed in, the kinds and factors offered, the state each kind kept
for its prompt, and whether it was a fresh sign-in on an existing
session.
*/
type challengeData struct {
	PrimaryMethod string                     `json:"primary_method"`
	Methods       []string                   `json:"methods"`
	FactorIDs     []string                   `json:"factor_ids"`
	DriverState   map[string]json.RawMessage `json:"driver_state"`
	Reauth        bool                       `json:"reauth"`
}

/*
challenged is a challenge being answered: its secret, what it
remembers, the user it is for and the client answering.
*/
type challenged struct {
	token  models.OneTimeToken
	data   challengeData
	user   models.User
	client appauth.Client
}

/*
CompleteTwoFactorChallenge signs a user in with a second factor.

Rules:
  - At most ten answers a minute from one address.
  - Only a live challenge is answered; the method must be one it
    offered, else nothing is counted.
  - The lockout is checked first; a wrong code counts toward it and
    toward the challenge, which five wrong codes burn.
  - The factor is locked, checked and updated in one transaction that
    also spends the challenge.
  - A recovery code is logged and mailed with the codes left.
  - The session records both methods, the primary one first.

Errors: [exceptions.ErrTwoFactorChallengeExpired],
[exceptions.ErrMethodNotAvailable], [exceptions.ErrInvalidCode],
[exceptions.ErrTooManyAttempts], [exceptions.ErrReauthUserMismatch],
those of [StartSession] and [LoadMe], or a wrapped error.
*/
type CompleteTwoFactorChallenge struct {
	DB           *database.DB
	Clock        clock.Clock
	Mail         *mail.Manager
	Limiter      *ratelimit.Limiter
	Links        appauth.Links
	Lockout      *appauth.Lockout
	Tokens       *appauth.OneTimeTokens
	Factors      *factors.Registry
	SecurityLog  *appauth.SecurityLog
	StartSession *StartSession
	LoadMe       *LoadMe
}

/*
Handle checks the code and starts the session.
*/
func (a *CompleteTwoFactorChallenge) Handle(ctx context.Context, in CompleteTwoFactorChallengeInput) (
	SignedIn, error,
) {
	err := appauth.Throttle(ctx, a.Limiter, appauth.Rate{
		Key: "two_factor:redeem:" + in.Client.IP, Limit: 10, Window: time.Minute,
	})
	if err != nil {
		return SignedIn{}, err
	}

	c, err := a.find(ctx, in.Token)
	if err != nil {
		return SignedIn{}, err
	}

	c.client = in.Client

	driver, ok := a.Factors.Find(in.Method)
	if !ok || !slices.Contains(c.data.Methods, in.Method) {
		return SignedIn{}, exceptions.ErrMethodNotAvailable
	}

	if c.data.Reauth && !isSessionUser(ctx, c.user) {
		return SignedIn{}, exceptions.ErrReauthUserMismatch
	}

	attempt := appauth.Attempt{IP: in.Client.IP, Email: c.user.Email, KnownDevice: true}
	if err := a.Lockout.Check(ctx, attempt); err != nil {
		return SignedIn{}, err
	}

	passed, err := a.verify(ctx, c, driver, in)
	if err != nil {
		return SignedIn{}, err
	}

	if !passed {
		return SignedIn{}, a.miss(ctx, c, attempt, wrongAnswer(in))
	}

	return a.signIn(ctx, c, in)
}

/*
find returns the live challenge of the token, with the user it is for.
*/
func (a *CompleteTwoFactorChallenge) find(ctx context.Context, token string) (challenged, error) {
	tok, ok, err := a.Tokens.FindByToken(ctx, enums.OneTimePurposeTwoFactorChallenge, token)
	if err != nil {
		return challenged{}, err
	}

	live := ok && tok.ConsumedAt == nil && a.Clock.Now().Before(tok.ExpiresAt) &&
		tok.Attempts < appauth.MaxSecretAttempts && tok.UserID != nil
	if !live {
		return challenged{}, exceptions.ErrTwoFactorChallengeExpired
	}

	c := challenged{token: tok}
	if err := json.Unmarshal(tok.Data, &c.data); err != nil {
		return challenged{}, fmt.Errorf("decode two-factor challenge: %w", err)
	}

	c.user, err = models.New(a.DB).FindUser(ctx, *tok.UserID)
	if err != nil {
		return challenged{}, fmt.Errorf("find user of two-factor challenge: %w", err)
	}

	if c.user.DeletedAt != nil {
		return challenged{}, exceptions.ErrTwoFactorChallengeExpired
	}

	return c, nil
}

/*
verify checks the code against the offered factors of the kind and,
when one accepts it, records the use and spends the challenge. It
reports whether a factor accepted the code.
*/
func (a *CompleteTwoFactorChallenge) verify(
	ctx context.Context, c challenged, driver factors.Factor, in CompleteTwoFactorChallengeInput,
) (bool, error) {
	passed := false

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		f, ok, err := a.match(ctx, c, driver, factors.Answer{
			Code: in.Code, Credential: in.Credential, State: c.data.DriverState[driver.Type()],
		})
		if err != nil || !ok {
			return err
		}

		passed = true

		if err := a.use(ctx, f); err != nil {
			return err
		}

		spent, err := a.Tokens.Spend(ctx, c.token)
		if err != nil {
			return err
		}

		if !spent {
			return exceptions.ErrTwoFactorChallengeExpired
		}

		if f.Type != factors.RecoveryCodeType {
			return nil
		}

		return a.recoveryCodeUsed(ctx, tx, c, f)
	})

	return passed, err
}

/*
match locks the user's factors and returns the offered one of the kind
that accepts the answer, and whether one did.
*/
func (a *CompleteTwoFactorChallenge) match(
	ctx context.Context, c challenged, driver factors.Factor, answer factors.Answer,
) (models.UserAuthFactor, bool, error) {
	held, err := models.New(a.DB).LockUserAuthFactors(ctx, c.user.ID)
	if err != nil {
		return models.UserAuthFactor{}, false, fmt.Errorf("lock factors to check code: %w", err)
	}

	for _, f := range held {
		offered := f.Type == driver.Type() && f.Status == string(enums.AuthFactorStatusActive) &&
			slices.Contains(c.data.FactorIDs, f.ID.String())
		if !offered {
			continue
		}

		err := driver.Verify(ctx, &f, answer)
		if err == nil {
			return f, true, nil
		}

		if !errors.Is(err, factors.ErrInvalidCode) && !errors.Is(err, factors.ErrCodeReused) {
			return models.UserAuthFactor{}, false, fmt.Errorf("check %s code: %w", f.Type, err)
		}
	}

	return models.UserAuthFactor{}, false, nil
}

/*
use stores what checking the code changed on the factor.
*/
func (a *CompleteTwoFactorChallenge) use(ctx context.Context, f models.UserAuthFactor) error {
	err := models.New(a.DB).RecordAuthFactorUse(ctx, models.RecordAuthFactorUseParams{
		ID: f.ID, SecretEncrypted: f.SecretEncrypted, Credential: f.Credential, LastCounter: f.LastCounter,
		Now: a.Clock.Now(),
	})
	if err != nil {
		return fmt.Errorf("record factor use: %w", err)
	}

	return nil
}

/*
recoveryCodeUsed logs the spent recovery code and mails the owner how
many are left.
*/
func (a *CompleteTwoFactorChallenge) recoveryCodeUsed(
	ctx context.Context, tx *database.Tx, c challenged, f models.UserAuthFactor,
) error {
	err := a.SecurityLog.Record(ctx, appauth.Entry{
		UserID: c.user.ID, Type: appauth.EventAuthFactorRecoveryCodeUsed, Client: c.client,
		Metadata: map[string]any{"factor_id": models.PublicID(models.PrefixAuthFactor, f.ID), "type": f.Type},
	})
	if err != nil {
		return err
	}

	err = appmail.ToUser(a.Mail, c.user).QueueTx(ctx, tx, appmail.RecoveryCodeUsedMail{
		Name: c.user.Name, Remaining: factors.CodesOf(f).Remaining, SecurityURL: a.Links.Security(),
	})
	if err != nil {
		return fmt.Errorf("queue recovery code used mail: %w", err)
	}

	return nil
}

/*
miss counts a wrong answer toward the lockout and the challenge, and
returns why it failed: the wrong answer, or a challenge it just burned.
*/
func (a *CompleteTwoFactorChallenge) miss(
	ctx context.Context, c challenged, attempt appauth.Attempt, wrong error,
) error {
	if _, err := a.Lockout.Failed(ctx, attempt); err != nil {
		return err
	}

	attempts, err := models.New(a.DB).CountOneTimeTokenAttempt(ctx, c.token.ID)
	if err != nil {
		return fmt.Errorf("count wrong two-factor code: %w", err)
	}

	if attempts >= appauth.MaxSecretAttempts {
		return exceptions.ErrTwoFactorChallengeExpired
	}

	return wrong
}

/*
wrongAnswer is the failure of an answer that did not pass: a signed
credential that did not verify, or a wrong code.
*/
func wrongAnswer(in CompleteTwoFactorChallengeInput) error {
	if in.Credential != nil {
		return exceptions.ErrInvalidCredential
	}

	return exceptions.ErrInvalidCode
}

/*
signIn starts the session with both methods and describes the caller.
*/
func (a *CompleteTwoFactorChallenge) signIn(ctx context.Context, c challenged, in CompleteTwoFactorChallengeInput) (
	SignedIn, error,
) {
	started, err := a.StartSession.Handle(ctx, StartSessionInput{
		User: c.user, Methods: []string{c.data.PrimaryMethod, in.Method}, Client: in.Client, Reauth: c.data.Reauth,
	})
	if err != nil {
		return SignedIn{}, err
	}

	me, err := a.LoadMe.Handle(ctx, LoadMeInput{Principal: started.Principal})
	if err != nil {
		return SignedIn{}, err
	}

	return SignedIn{Me: me, DeviceToken: started.DeviceToken}, nil
}
