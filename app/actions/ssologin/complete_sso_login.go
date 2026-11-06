/*
|--------------------------------------------------------------------------
| Complete SSO Login
|--------------------------------------------------------------------------
|
| A company's identity provider vouches for a person; this decides who
| they are here and lets them in. A subject already linked is that user.
| Otherwise the address must sit under a domain the company verified and
| routed to the connection, so one company's provider can never sign in
| another's people. An existing account at that address is proven and
| linked; a new one is made only when the connection admits newcomers.
| The person must be an active member of the company's workspace, or
| become one when the connection admits newcomers; a deactivated member
| stays out. The provider has proven who they are, so no second factor
| is asked for.
|
*/

package ssologin

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/actions/auth"
	"github.com/dimmihq/dimmi/app/actions/sso"
	"github.com/dimmihq/dimmi/app/actions/users"
	appauth "github.com/dimmihq/dimmi/app/auth"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
loginTouchEvery is how often a connection's last sign-in is written.
*/
const loginTouchEvery = 5 * time.Minute

/*
Identity is what an identity provider reports about the person: their
subject, address, name and groups.
*/
type Identity struct {
	Subject string
	Email   string
	Name    string
	Groups  []string
}

/*
CompleteSSOLoginInput is the connection the person came through, what
it reports about them, and the client they use.
*/
type CompleteSSOLoginInput struct {
	Connection sso.Connection
	Identity   Identity
	Client     appauth.Client
}

/*
CompleteSSOLogin signs a person in through a connection.

Errors: [exceptions.ErrSSOEmailDomainNotVerified],
[exceptions.ErrSSONoAccess], [exceptions.ErrMembershipDeactivated], or a
wrapped error.
*/
type CompleteSSOLogin struct {
	DB                  *database.DB
	Clock               clock.Clock
	Events              *events.Dispatcher
	CreateUser          *users.CreateUser
	ClaimAccount        *auth.ClaimAccount
	CompletePrimaryAuth *auth.CompletePrimaryAuth
}

/*
Handle resolves the person, admits them to the workspace and starts
their session.
*/
func (a *CompleteSSOLogin) Handle(ctx context.Context, in CompleteSSOLoginInput) (auth.SignedIn, error) {
	var user models.User

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error

		user, err = a.resolve(ctx, tx, in)

		return err
	})
	if err != nil {
		return auth.SignedIn{}, err
	}

	if err := a.admit(ctx, in, user); err != nil {
		return auth.SignedIn{}, err
	}

	now := a.Clock.Now()
	if err := models.New(a.DB).TouchSSOConnectionLogin(ctx, models.TouchSSOConnectionLoginParams{
		Now: now, ID: in.Connection.Connection.ID, Since: now.Add(-loginTouchEvery),
	}); err != nil {
		return auth.SignedIn{}, fmt.Errorf("record connection sign-in: %w", err)
	}

	return a.CompletePrimaryAuth.Handle(ctx, auth.CompletePrimaryAuthInput{
		User: user, Method: appauth.MethodSSO, Client: in.Client, SkipTwoFactor: true,
		Context: auth.SignInContext{ConnectionID: in.Connection.Connection.ID, Domain: domainOf(in.Identity.Email)},
	})
}

/*
resolve returns the user the connection's subject is, linking or making
the account the first time.
*/
func (a *CompleteSSOLogin) resolve(
	ctx context.Context, tx *database.Tx, in CompleteSSOLoginInput,
) (models.User, error) {
	q := models.New(tx)
	row := in.Connection.Connection
	now := a.Clock.Now()
	email := strings.ToLower(strings.TrimSpace(in.Identity.Email))

	linked, err := q.FindSSOIdentity(ctx, models.FindSSOIdentityParams{
		SsoConnectionID: &row.ID, Subject: in.Identity.Subject,
	})
	if err == nil {
		touch := models.TouchSSOIdentityParams{Email: &email, Now: &now, ID: linked.ID}
		if err := q.TouchSSOIdentity(ctx, touch); err != nil {
			return models.User{}, fmt.Errorf("touch sso identity: %w", err)
		}

		return a.manage(ctx, q, linked.UserID, in)
	}

	if !errors.Is(err, database.ErrNotFound) {
		return models.User{}, fmt.Errorf("find sso identity: %w", err)
	}

	_, err = q.FindRoutedDomain(ctx, models.FindRoutedDomainParams{
		OrganizationID: row.OrganizationID, Domain: domainOf(email), SsoConnectionID: &row.ID,
	})
	if errors.Is(err, database.ErrNotFound) {
		return models.User{}, exceptions.ErrSSOEmailDomainNotVerified
	}

	if err != nil {
		return models.User{}, fmt.Errorf("find routed domain: %w", err)
	}

	user, err := a.account(ctx, tx, in, email)
	if err != nil {
		return models.User{}, err
	}

	if _, err := q.CreateSSOIdentity(ctx, models.CreateSSOIdentityParams{
		UserID: user.ID, Subject: in.Identity.Subject, SsoConnectionID: &row.ID, Email: &email, Now: &now,
	}); err != nil {
		return models.User{}, fmt.Errorf("link sso identity: %w", err)
	}

	return a.manage(ctx, q, user.ID, in)
}

/*
account returns the account at the address, proven, or makes one when
the connection admits newcomers.
*/
func (a *CompleteSSOLogin) account(
	ctx context.Context, tx *database.Tx, in CompleteSSOLoginInput, email string,
) (models.User, error) {
	existing, err := models.New(tx).FindUserByEmail(ctx, email)
	if err == nil {
		return a.ClaimAccount.Prove(ctx, tx, existing, in.Client)
	}

	if !errors.Is(err, database.ErrNotFound) {
		return models.User{}, fmt.Errorf("find account by address: %w", err)
	}

	if !in.Connection.Connection.JitEnabled {
		return models.User{}, exceptions.ErrSSONoAccess
	}

	user, err := a.CreateUser.Handle(ctx, users.CreateUserInput{
		Name: cmp.Or(strings.TrimSpace(in.Identity.Name), strings.Split(email, "@")[0]), Email: email,
		Verified: true, CreatedVia: appauth.MethodSSO,
	})
	if err != nil {
		return models.User{}, err
	}

	return user, a.Events.Dispatch(ctx, tx, appevents.UserRegistered{UserID: user.ID})
}

/*
manage hands the account to the company and keeps its name in step with
what the identity provider reports.
*/
func (a *CompleteSSOLogin) manage(
	ctx context.Context, q *models.Queries, userID ids.UUID, in CompleteSSOLoginInput,
) (models.User, error) {
	var name *string
	if trimmed := strings.TrimSpace(in.Identity.Name); trimmed != "" {
		name = &trimmed
	}

	organization := in.Connection.Connection.OrganizationID

	user, err := q.SetUserManagedBy(ctx, models.SetUserManagedByParams{
		OrganizationID: &organization, Name: name, Now: a.Clock.Now(), ID: userID,
	})
	if err != nil {
		return models.User{}, fmt.Errorf("manage account: %w", err)
	}

	return user, nil
}

/*
admit makes sure the user is an active member of the company's
workspace, adding them when the connection admits newcomers, and applies
the roles their groups give.
*/
func (a *CompleteSSOLogin) admit(ctx context.Context, in CompleteSSOLoginInput, user models.User) error {
	workspace, err := models.New(a.DB).FindOrganizationWorkspace(ctx, in.Connection.Connection.OrganizationID)
	if errors.Is(err, database.ErrNotFound) {
		return exceptions.ErrSSONoAccess
	}

	if err != nil {
		return fmt.Errorf("find company workspace: %w", err)
	}

	scoped := tenancy.With(ctx, workspace.ID.String())

	return a.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
		member, err := a.membership(ctx, joining{tx: tx, in: in, workspace: workspace, user: user})
		if err != nil {
			return err
		}

		return a.applyGroups(ctx, tx, in, member)
	})
}

/*
domainOf returns the domain of the address, lowercase.
*/
func domainOf(email string) string {
	at := strings.LastIndex(email, "@")
	if at < 0 {
		return ""
	}

	return strings.ToLower(email[at+1:])
}
