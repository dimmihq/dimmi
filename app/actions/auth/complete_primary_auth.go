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
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"slices"
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
sign-in on an existing session. Context says where the sign-in came from
when an outside service vouched for it. SkipTwoFactor starts the session
without a challenge, for a company's identity provider that already
proved who the person is. SSOBypass records that an owner signed in
around their company's identity provider, through the challenge too.
*/
type CompletePrimaryAuthInput struct {
	User    models.User
	Method  string
	Client  appauth.Client
	Reauth  bool
	Context SignInContext

	SkipTwoFactor bool
	SSOBypass     bool
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
the factor, its name, whether it is the user's preferred one, and the
options the browser needs to answer with it, nil for typed codes.
*/
type ChallengeMethod struct {
	Method    string
	FactorID  ids.UUID
	Name      string
	Preferred bool
	Options   json.RawMessage
}

/*
CompletePrimaryAuth signs in a user who passed a primary method.

Rules:
  - A user with two-factor authentication on gets a challenge instead
    of a session, offering their active factors of enabled kinds that
    answer challenges, preferred first and recovery codes last; a
    used-up recovery set is left out.
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
	if in.User.TwoFactorEnabled() && !in.SkipTwoFactor {
		challenge, err := a.challenge(ctx, in)

		return SignedIn{Challenge: &challenge}, err
	}

	started, err := a.StartSession.Handle(ctx, StartSessionInput{
		User: in.User, Methods: signInMethods{[]string{in.Method}, in.SSOBypass}.list(), Client: in.Client,
		Reauth: in.Reauth, Context: in.Context,
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
it. Each kind that needs a prompt gets one, shared by its factors, and
the state the kind keeps rides on the challenge.
*/
func (a *CompletePrimaryAuth) challenge(ctx context.Context, in CompletePrimaryAuthInput) (TwoFactorChallenge, error) {
	offered, err := a.offered(ctx, in.User)
	if err != nil {
		return TwoFactorChallenge{}, err
	}

	prompts, err := a.prompts(ctx, in.User, offered)
	if err != nil {
		return TwoFactorChallenge{}, err
	}

	methods, kinds, factorIDs := []ChallengeMethod{}, []string{}, []string{}
	states := map[string]json.RawMessage{}

	for _, f := range offered {
		kinds = append(kinds, f.Type)
		factorIDs = append(factorIDs, f.ID.String())
		methods = append(methods, ChallengeMethod{
			Method: f.Type, FactorID: f.ID, Name: f.Name, Preferred: f.IsPreferred, Options: prompts[f.Type].Options,
		})

		if state := prompts[f.Type].State; state != nil {
			states[f.Type] = state
		}
	}

	issued, err := a.Tokens.Issue(ctx, appauth.Issue{
		Purpose: enums.OneTimePurposeTwoFactorChallenge, UserID: &in.User.ID, Email: in.User.Email,
		Data: map[string]any{
			"primary_method": in.Method, "methods": kinds, "factor_ids": factorIDs, "reauth": in.Reauth,
			"driver_state": states, "auth_domain": in.Context.Domain, "sso_bypass": in.SSOBypass,
		},
		Client: in.Client, TTL: challengeTTL, Live: appauth.MaxSecretAttempts,
	})
	if err != nil {
		return TwoFactorChallenge{}, err
	}

	return TwoFactorChallenge{Token: issued.Token, ExpiresAt: issued.ExpiresAt, Methods: methods}, nil
}

/*
offered returns the factors that may answer a challenge for the user,
preferred first and recovery codes last.
*/
func (a *CompletePrimaryAuth) offered(ctx context.Context, user models.User) ([]models.UserAuthFactor, error) {
	held, err := models.New(a.DB).ListActiveAuthFactors(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("list factors to challenge: %w", err)
	}

	offered := []models.UserAuthFactor{}

	for _, f := range held {
		driver, ok := a.Factors.Find(f.Type)
		if !ok || !driver.Capabilities().SecondFactor {
			continue
		}

		if f.Type == factors.RecoveryCodeType && factors.CodesOf(f).Remaining == 0 {
			continue
		}

		offered = append(offered, f)
	}

	slices.SortStableFunc(offered, func(x, y models.UserAuthFactor) int {
		return cmp.Compare(a.standIn(x), a.standIn(y))
	})

	return offered, nil
}

/*
standIn ranks a factor that only stands in for another, such as a
recovery code, after the real ones.
*/
func (a *CompletePrimaryAuth) standIn(f models.UserAuthFactor) int {
	if a.Factors.EnablesTwoFactor(f.Type) {
		return 0
	}

	return 1
}

/*
prompts asks each offered kind what its factors need to answer.
*/
func (a *CompletePrimaryAuth) prompts(
	ctx context.Context, user models.User, offered []models.UserAuthFactor,
) (map[string]factors.Prompt, error) {
	byKind := map[string][]models.UserAuthFactor{}
	for _, f := range offered {
		byKind[f.Type] = append(byKind[f.Type], f)
	}

	prompts := map[string]factors.Prompt{}

	for kind, held := range byKind {
		driver, _ := a.Factors.Find(kind)

		prompt, err := factors.Challenge(ctx, driver, user, held)
		if err != nil {
			return nil, err
		}

		prompts[kind] = prompt
	}

	return prompts, nil
}

/*
signInMethods returns the methods of a sign-in, with the record of an
owner's sign-in around the identity provider when there was one.
*/
type signInMethods struct {
	methods []string
	bypass  bool
}

/*
list returns the methods to record.
*/
func (m signInMethods) list() []string {
	if !m.bypass {
		return m.methods
	}

	return append(m.methods, appauth.MethodSSOBypass)
}
