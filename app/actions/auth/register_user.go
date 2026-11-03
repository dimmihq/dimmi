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
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/ratelimit"

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
its first workspace, with the client signing up.
*/
type RegisterUserInput struct {
	Name          string
	Email         string
	Password      string
	WorkspaceName string
	Client        appauth.Client
}

/*
RegisterUser creates an account and signs it in.

Rules:
  - The login policy for the address is asked first.
  - Only an open install takes new accounts.
  - One address creates a limited number of accounts per hour.
  - A workspace name creates a workspace the new user owns.
  - The new address is sent a verification link; nothing waits for it.

Events: [appevents.UserRegistered], and those of
[workspaces.CreateWorkspace] when a workspace is created.

Errors: [exceptions.ErrSignupClosed], [exceptions.ErrTooManySignups],
[exceptions.ErrEmailTaken], [exceptions.ErrSSORequired],
[exceptions.ErrAuthBusy], or a wrapped error.
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
}

/*
Handle registers the user and starts their session.
*/
func (a *RegisterUser) Handle(ctx context.Context, in RegisterUserInput) (SignedIn, error) {
	if err := a.admit(ctx, in); err != nil {
		return SignedIn{}, err
	}

	hash, err := a.Hash.Make(ctx, in.Password)
	if errors.Is(err, hashing.ErrBusy) {
		return SignedIn{}, exceptions.ErrAuthBusy
	}

	if err != nil {
		return SignedIn{}, fmt.Errorf("hash password: %w", err)
	}

	user, workspaceID, err := a.create(ctx, in, hash)
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
admit decides whether this signup may go ahead at all.
*/
func (a *RegisterUser) admit(ctx context.Context, in RegisterUserInput) error {
	if err := appauth.CheckLoginPolicy(ctx, a.LoginPolicy, in.Email, appauth.MethodPassword); err != nil {
		return err
	}

	if a.Signup.Mode != config.SignupOpen {
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
create writes the account and, when asked, its workspace, in one
transaction. It returns the workspace id, or ids.Nil() for none.
*/
func (a *RegisterUser) create(ctx context.Context, in RegisterUserInput, hash string) (models.User, ids.UUID, error) {
	var (
		user        models.User
		workspaceID = ids.Nil()
	)

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error

		user, err = a.CreateUser.Handle(ctx, users.CreateUserInput{
			Name: in.Name, Email: in.Email, PasswordHash: &hash, CreatedVia: appauth.CreatedViaRegister,
		})
		if err != nil {
			return err
		}

		if err := a.SendVerification.Handle(ctx, user, in.Client); err != nil {
			return err
		}

		if in.WorkspaceName != "" {
			created, err := a.CreateWorkspace.Handle(ctx, workspaces.CreateWorkspaceInput{
				Owner: user, Name: in.WorkspaceName, JoinedVia: appauth.JoinedViaCreated,
			})
			if err != nil {
				return err
			}

			workspaceID = created.Workspace.ID
		}

		return a.Events.Dispatch(ctx, tx, appevents.UserRegistered{UserID: user.ID})
	})

	return user, workspaceID, err
}
