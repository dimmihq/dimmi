/*
|--------------------------------------------------------------------------
| Complete Primary Auth
|--------------------------------------------------------------------------
|
| Every way of proving who you are on its own, a password, a mailed link
| or code, a provider, a reset, ends here, so the rule about second
| factors lives in one place: an account protected by one is never
| signed in on the first proof alone, whichever proof it was.
|
| Instead the browser gets a challenge: a short-lived secret that says
| who passed the first step and which factors may finish it. Two-factor
| authentication never switches itself off: when no factor can answer,
| because its kind was disabled or the recovery codes ran out, the
| challenge still comes, offering nothing, and only an operator can let
| the user in.
|
*/

package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/auth/factors"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
challengeTTL is how long a second-factor challenge may be answered.
*/
const challengeTTL = 5 * time.Minute

/*
CompletePrimaryAuthInput is the user who just proved who they are, the
method they proved it with, and the client. Reauth marks a fresh
sign-in on an existing session.
*/
type CompletePrimaryAuthInput struct {
	User   models.User
	Method string
	Client appauth.Client
	Reauth bool
}

/*
TwoFactorChallenge is the second step a sign-in waits on: the secret to
hand the browser, when it runs out, and the factors that may answer.
*/
type TwoFactorChallenge struct {
	Token     string
	ExpiresAt time.Time
	Methods   []ChallengeMethod
}

/*
ChallengeMethod is one factor that may answer a challenge: its kind,
the factor, its name and whether it is the user's preferred one.
*/
type ChallengeMethod struct {
	Method    string
	FactorID  ids.UUID
	Name      string
	Preferred bool
}

/*
CompletePrimaryAuth signs in a user who passed a primary method.

Rules:
  - A user with two-factor authentication on gets a challenge instead
    of a session, offering their active factors of enabled kinds that
    answer challenges, preferred first; a used-up recovery set is left
    out.
  - Otherwise a session starts with the method as its only one.

Errors: those of [StartSession] and [LoadMe], or a wrapped error.
*/
type CompletePrimaryAuth struct {
	DB           *database.DB
	Tokens       *appauth.OneTimeTokens
	Factors      *factors.Registry
	StartSession *StartSession
	LoadMe       *LoadMe
}

/*
Handle starts the session and describes the caller, or begins the
challenge.
*/
func (a *CompletePrimaryAuth) Handle(ctx context.Context, in CompletePrimaryAuthInput) (SignedIn, error) {
	if in.User.TwoFactorEnabled() {
		challenge, err := a.challenge(ctx, in)

		return SignedIn{Challenge: &challenge}, err
	}

	started, err := a.StartSession.Handle(ctx, StartSessionInput{
		User: in.User, Methods: []string{in.Method}, Client: in.Client, Reauth: in.Reauth,
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

/*
challenge stores a challenge for the user's usable factors and returns
it.
*/
func (a *CompletePrimaryAuth) challenge(ctx context.Context, in CompletePrimaryAuthInput) (TwoFactorChallenge, error) {
	methods, err := a.methods(ctx, in.User)
	if err != nil {
		return TwoFactorChallenge{}, err
	}

	kinds := []string{}
	factorIDs := []string{}

	for _, m := range methods {
		kinds = append(kinds, m.Method)
		factorIDs = append(factorIDs, m.FactorID.String())
	}

	issued, err := a.Tokens.Issue(ctx, appauth.Issue{
		Purpose: enums.OneTimePurposeTwoFactorChallenge, UserID: &in.User.ID, Email: in.User.Email,
		Data: map[string]any{
			"primary_method": in.Method, "methods": kinds, "factor_ids": factorIDs, "reauth": in.Reauth,
		},
		Client: in.Client, TTL: challengeTTL, Live: appauth.MaxSecretAttempts,
	})
	if err != nil {
		return TwoFactorChallenge{}, err
	}

	return TwoFactorChallenge{Token: issued.Token, ExpiresAt: issued.ExpiresAt, Methods: methods}, nil
}

/*
methods returns the factors that may answer a challenge for the user.
*/
func (a *CompletePrimaryAuth) methods(ctx context.Context, user models.User) ([]ChallengeMethod, error) {
	held, err := models.New(a.DB).ListActiveAuthFactors(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("list factors to challenge: %w", err)
	}

	methods := []ChallengeMethod{}

	for _, f := range held {
		driver, ok := a.Factors.Find(f.Type)
		if !ok || !driver.Capabilities().SecondFactor {
			continue
		}

		if f.Type == factors.RecoveryCodeType && factors.CodesOf(f).Remaining == 0 {
			continue
		}

		methods = append(methods, ChallengeMethod{
			Method: f.Type, FactorID: f.ID, Name: f.Name, Preferred: f.IsPreferred,
		})
	}

	return methods, nil
}
