/*
|--------------------------------------------------------------------------
| Session Controller
|--------------------------------------------------------------------------
|
| Sessions are the resource of signing in and out: POST creates one with
| a grant, GET lists the user's own, DELETE ends one, "current" or
| another by id, and DELETE on the collection ends every other one.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/auth"
	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
SessionsOutput is the list of the user's sessions.
*/
type SessionsOutput struct {
	Body fhttp.Collection[resources.Session]
}

/*
SessionController serves the session collection.
*/
type SessionController struct {
	AttemptLogin     *auth.AttemptLogin
	ListSessions     *auth.ListSessions
	EndSession       *auth.EndSession
	EndOtherSessions *auth.EndOtherSessions
	IDs              *ids.Registry
	Cookies          appauth.Cookies
	Server           Server
}

/*
Store signs in with the grant and answers with the caller's picture.
*/
func (c *SessionController) Store(ctx context.Context, in *requests.StoreSessionRequest) (*MeOutput, error) {
	if in.Body.Grant != requests.GrantPassword {
		return nil, exceptions.ErrGrantNotAllowed
	}

	signedIn, err := c.AttemptLogin.Handle(ctx, auth.AttemptLoginInput{
		Email:    in.Body.Email,
		Password: in.Body.Password,
		Reauth:   in.Body.Reauth,
		Client:   clientOf(ctx, c.Cookies),
	})
	if err != nil {
		return nil, err
	}

	return &MeOutput{
		SetCookie: deviceCookies(c.Cookies, signedIn.DeviceToken),
		Body:      fhttp.NewResource(resources.NewMe(signedIn.Me, c.Server.instance())),
	}, nil
}

/*
Index lists the caller's browser sessions.
*/
func (c *SessionController) Index(ctx context.Context, _ *requests.NoInput) (*SessionsOutput, error) {
	p := caller(ctx)

	sessions, err := c.ListSessions.Handle(ctx, p)
	if err != nil {
		return nil, err
	}

	page := pagination.Page[resources.Session]{Items: resources.NewSessions(sessions), Limit: len(sessions)}

	return &SessionsOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
Destroy ends one of the caller's sessions, the current one included.
*/
func (c *SessionController) Destroy(ctx context.Context, in *requests.DestroySessionRequest) (*EmptyOutput, error) {
	p := caller(ctx)

	id := p.CredentialID
	if in.Session != requests.CurrentSession {
		var err error

		id, err = c.IDs.Decode(string(models.PrefixSession), in.Session)
		if err != nil {
			return nil, exceptions.ErrNotFound
		}
	}

	err := c.EndSession.Handle(ctx, auth.EndSessionInput{Principal: p, SessionID: id, Client: clientOf(ctx, c.Cookies)})
	if err != nil {
		return nil, err
	}

	return &EmptyOutput{}, nil
}

/*
DestroyMany ends every session of the caller but the current one.
*/
func (c *SessionController) DestroyMany(ctx context.Context, _ *requests.NoInput) (*EmptyOutput, error) {
	p := caller(ctx)

	if err := c.EndOtherSessions.Handle(ctx, p, clientOf(ctx, c.Cookies)); err != nil {
		return nil, err
	}

	return &EmptyOutput{}, nil
}
