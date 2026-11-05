/*
|--------------------------------------------------------------------------
| Register User
|--------------------------------------------------------------------------
|
| Signing up with an email and a password, and optionally naming a first
| workspace, in one request: the person lands signed in, in their new
| workspace, without waiting for any mail. The account, the workspace
| and its organization are written in one transaction, so a failure
| leaves nothing half made, and two sign-ups racing for one address
| leave exactly one account.
|
| Unlike signing in, registering says openly that an address is taken;
| it is throttled per address instead, so it cannot be used to sweep for
| accounts.
|
*/

package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/ratelimit"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/actions/invitations"
	"github.com/dimmihq/dimmi/app/actions/users"
	"github.com/dimmihq/dimmi/app/actions/workspaces"
	appauth "github.com/dimmihq/dimmi/app/auth"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/config"
)

/*
signupWindow is the period the per-address signup cap counts in.
*/
const signupWindow = time.Hour

/*
RegisterUserInput is the new account and, when WorkspaceName is set,
its first workspace, with the client signing up. InvitationToken is the
link of an invitation the account answers, "" for none.
*/
type RegisterUserInput struct {
	Name            string
	Email           string
	Password        string
	WorkspaceName   string
	InvitationToken string
	CaptchaToken    string
	Client          appauth.Client
}

/*
RegisterUser creates an account and signs it in.

Rules:
  - The login policy for the address is asked first, then the captcha,
    when the server asks one.
  - Only an open install takes new accounts, unless the person holds a
    pending invitation to the address they sign up with.
  - One address creates a limited number of accounts per hour.
  - An invitation makes the account a member of its workspace, and its
    address verified; no workspace is created then.
  - Otherwise a workspace name creates a workspace the new user owns.
  - The new address is sent a verification link unless the invitation
    proved it; nothing waits for it.

Events: [appevents.UserRegistered], and those of
[workspaces.CreateWorkspace] or [invitations.AcceptInvitation].

Errors: [exceptions.ErrCaptchaFailed], [exceptions.ErrCaptchaUnavailable],
[exceptions.ErrSignupClosed], [exceptions.ErrTooManySignups],
[exceptions.ErrEmailTaken], [exceptions.ErrSSORequired],
[exceptions.ErrAuthBusy], [exceptions.ErrInvitationEmailMismatch], the
errors of [invitations.AcceptInvitation], or a wrapped error.
*/
type RegisterUser struct {
	DB               *database.DB
	Hash             *hashing.Manager
	Events           *events.Dispatcher
	Limiter          *ratelimit.Limiter
	Signup           config.SignupConfig
	LoginPolicy      appauth.LoginPolicyResolver
	CreateUser       *users.CreateUser
	CreateWorkspace  *workspaces.CreateWorkspace
	SendVerification *SendEmailVerification
	StartSession     *StartSession
	LoadMe           *LoadMe
	Preview          *invitations.PreviewInvitation
	Accept           *invitations.AcceptInvitation
	Captcha          appauth.Captcha
	Risk             *appauth.SignupRisk
	Clock            clock.Clock
}

/*
Handle registers the user and starts their session.
*/
func (a *RegisterUser) Handle(ctx context.Context, in RegisterUserInput) (SignedIn, error) {
	invited, err := a.invitation(ctx, in)
	if err != nil {
		return SignedIn{}, err
	}

	if err := a.admit(ctx, in, invited); err != nil {
		return SignedIn{}, err
	}

	marks, err := a.marks(ctx, in, invited)
	if err != nil {
		return SignedIn{}, err
	}

	hash, err := a.Hash.Make(ctx, in.Password)
	if errors.Is(err, hashing.ErrBusy) {
		return SignedIn{}, exceptions.ErrAuthBusy
	}

	if err != nil {
		return SignedIn{}, fmt.Errorf("hash password: %w", err)
	}

	user, workspaceID, err := a.create(ctx, signup{in: in, hash: hash, invited: invited, marks: marks})
	if err != nil {
		return SignedIn{}, err
	}

	started, err := a.StartSession.Handle(ctx, StartSessionInput{
		User: user, Methods: []string{appauth.MethodPassword}, Client: in.Client,
	})
	if err != nil {
		return SignedIn{}, err
	}

	me, err := a.LoadMe.Handle(ctx, LoadMeInput{Principal: started.Principal, WorkspaceID: workspaceID})
	if err != nil {
		return SignedIn{}, err
	}

	return SignedIn{Me: me, DeviceToken: started.DeviceToken}, nil
}

/*
admit decides whether this signup may go ahead at all. The invitation
it answers, nil for none, lets it through where signup is closed.
*/
func (a *RegisterUser) admit(ctx context.Context, in RegisterUserInput, invited *invitations.Preview) error {
	if err := appauth.CheckLoginPolicy(ctx, a.LoginPolicy, in.Email, appauth.MethodPassword); err != nil {
		return err
	}

	if err := appauth.CheckCaptcha(ctx, a.Captcha, in.CaptchaToken, in.Client.IP); err != nil {
		return err
	}

	if a.Signup.Mode != config.SignupOpen && invited == nil {
		return exceptions.ErrSignupClosed
	}

	res, err := a.Limiter.Attempt(ctx, "signup:ip:"+in.Client.IP, a.Signup.MaxPerHourPerIP, signupWindow)
	if err != nil {
		return appauth.Unavailable(ctx, err)
	}

	if !res.Allowed {
		return exceptions.ErrTooManySignups.With("retry_after", max(int(res.RetryAfter.Seconds()), 1))
	}

	return nil
}

/*
invitation returns the invitation the signup answers, nil for none,
refusing one that is not pending or not addressed to the email signing
up.
*/
func (a *RegisterUser) invitation(ctx context.Context, in RegisterUserInput) (*invitations.Preview, error) {
	if in.InvitationToken == "" {
		return nil, nil
	}

	preview, err := a.Preview.Handle(ctx, in.InvitationToken)
	if err != nil {
		return nil, err
	}

	if !strings.EqualFold(preview.Invitation.Email, strings.TrimSpace(in.Email)) {
		return nil, exceptions.ErrInvitationEmailMismatch
	}

	return &preview, nil
}

/*
create writes the account and, as asked, joins the workspace it was
invited to or creates its first workspace, in one transaction. It
returns the workspace id, or ids.Nil() for none. An invited account's
transaction works in the invitation's workspace.
*/
func (a *RegisterUser) create(ctx context.Context, s signup) (models.User, ids.UUID, error) {
	in, hash, invited := s.in, s.hash, s.invited

	var (
		user        models.User
		workspaceID = ids.Nil()
	)

	if invited != nil {
		ctx = tenancy.With(ctx, invited.Invitation.WorkspaceID.String())
	}

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error

		user, err = a.CreateUser.Handle(ctx, users.CreateUserInput{
			Name: in.Name, Email: in.Email, PasswordHash: &hash, Verified: invited != nil,
			CreatedVia: createdVia(invited),
		})
		if err != nil {
			return err
		}

		if err := a.mark(ctx, tx, user, s.marks); err != nil {
			return err
		}

		workspaceID, err = a.join(ctx, in, user)
		if err != nil {
			return err
		}

		return a.Events.Dispatch(ctx, tx, appevents.UserRegistered{UserID: user.ID})
	})

	return user, workspaceID, err
}

/*
signup is a checked signup: the request, the password's hash, the
invitation it answers, nil for none, and the marks it earned.
*/
type signup struct {
	in      RegisterUserInput
	hash    string
	invited *invitations.Preview
	marks   []string
}

/*
marks judges the signup's risk. An invited person is never judged: the
member who invited them vouched for the address.
*/
func (a *RegisterUser) marks(
	ctx context.Context, in RegisterUserInput, invited *invitations.Preview,
) ([]string, error) {
	if invited != nil {
		return nil, nil
	}

	return a.Risk.Assess(ctx, in.Email, in.Client.IP)
}

/*
mark records the marks the signup earned on the new account.
*/
func (a *RegisterUser) mark(ctx context.Context, tx *database.Tx, user models.User, marks []string) error {
	if len(marks) == 0 {
		return nil
	}

	err := models.New(tx).AddUserRiskFlags(ctx, models.AddUserRiskFlagsParams{
		ID: user.ID, Flags: marks, Now: a.Clock.Now(),
	})
	if err != nil {
		return fmt.Errorf("mark signup: %w", err)
	}

	return nil
}

/*
join accepts the invitation the new account answers, or else sends the
verification link and creates the workspace the person named. It
returns the workspace the account lands in, ids.Nil() for none.
*/
func (a *RegisterUser) join(ctx context.Context, in RegisterUserInput, user models.User) (ids.UUID, error) {
	if in.InvitationToken != "" {
		joined, err := a.Accept.Handle(ctx, invitations.AcceptInvitationInput{User: user, Token: in.InvitationToken})

		return joined.Workspace.ID, err
	}

	if err := a.SendVerification.Handle(ctx, user, in.Client); err != nil {
		return ids.Nil(), err
	}

	if in.WorkspaceName == "" {
		return ids.Nil(), nil
	}

	created, err := a.CreateWorkspace.Handle(ctx, workspaces.CreateWorkspaceInput{
		Owner: user, Name: in.WorkspaceName, JoinedVia: appauth.JoinedViaCreated,
	})

	return created.Workspace.ID, err
}

/*
createdVia returns how the account came to be: through an invitation or
by registering.
*/
func createdVia(invited *invitations.Preview) string {
	if invited != nil {
		return appauth.MethodInvitation
	}

	return appauth.CreatedViaRegister
}
