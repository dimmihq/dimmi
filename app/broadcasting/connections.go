/*
|--------------------------------------------------------------------------
| Realtime Connections
|--------------------------------------------------------------------------
|
| A realtime connection is authenticated by the same guards as the API
| when it opens. The dashboard rides on its session cookie, which
| only works when the browser opened the connection from the app's own
| origin: any site can make a browser open a socket and send the cookie
| along, so a cross-origin upgrade is refused before the cookie is read.
| Apps, the command line and integrations send their bearer token in the
| connect command instead, never in the URL, and then the cookie is
| ignored, so a token never borrows a browser's session.
|
| The connection belongs to the user. Which workspaces and channels it
| may follow is decided channel by channel when it subscribes, so one
| browser tab follows whichever workspace it shows, and losing access to
| a workspace closes the user's connections. It also remembers the session
| or token it opened with, and ends when that does.
|
*/

package broadcasting

import (
	"context"
	"errors"
	"fmt"

	fauth "github.com/dimmihq/dimmi/framework/auth"
	fbroadcasting "github.com/dimmihq/dimmi/framework/broadcasting"
	"github.com/dimmihq/dimmi/framework/fault"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/middleware"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/session"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Connections authenticates realtime connections. Guards are the API's
guards, Sessions loads the browser's session from its cookie and
Origins refuses cookie connections opened from another origin. Live says
whether a connection's credential still works.
*/
type Connections struct {
	Guards   *fauth.Guards
	Sessions *session.Manager
	Origins  *middleware.OriginCheck
	Live     *appauth.LiveCredentials
}

/*
Info is what other listeners learn about a connection, in presence
lists and join notices: the user's name and avatar.
*/
type Info struct {
	Name      string  `json:"name"`
	AvatarURL *string `json:"avatar_url"`
}

/*
Resolve returns the user behind a connection: by the bearer token in
the connect command when there is one, otherwise by the session cookie
of a same-origin upgrade. Missing, invalid and cross-origin credentials
are refused as unauthenticated.
*/
func (c *Connections) Resolve(ctx context.Context, r *fhttp.Request, token string) (fbroadcasting.Principal, error) {
	authenticated, err := c.authenticate(ctx, r, token)
	if err != nil {
		return fbroadcasting.Principal{}, err
	}

	principal, ok := authenticated.(*appauth.Principal)
	if !ok {
		return fbroadcasting.Principal{}, fbroadcasting.ErrUnauthenticated
	}

	return fbroadcasting.Principal{
		UserID:     UserID(principal),
		Info:       Info{Name: principal.User.Name, AvatarURL: principal.User.AvatarURL()},
		Subject:    principal,
		Credential: Credential(principal.Kind, principal.CredentialID),
		Alive:      func(ctx context.Context) (bool, error) { return c.Live.Alive(ctx, principal) },
	}, nil
}

/*
Credential is the name a connection's session or token goes by, which
is how the connections opened with it are closed when it ends.
*/
func Credential(kind string, id ids.UUID) string {
	return kind + ":" + id.String()
}

/*
UserID is the id a connection is known by, the user's public id, which
is also how every connection of the user is closed at once.
*/
func UserID(principal *appauth.Principal) string {
	return models.PublicID(models.PrefixUser, principal.User.ID)
}

/*
authenticate runs the guards on a copy of the opening request that
carries only the credential in play: the token as a bearer header, or
the cookie of the browser's session.
*/
func (c *Connections) authenticate(ctx context.Context, r *fhttp.Request, token string) (fauth.Principal, error) {
	req := r.Clone(ctx)

	if token != "" {
		req.Header.Del("Cookie")
		req.Header.Set("Authorization", "Bearer "+token)

		return refused(c.Guards.Authenticate(req))
	}

	req.Header.Del("Authorization")

	if err := c.Origins.Check(req); err != nil {
		return nil, fbroadcasting.ErrUnauthenticated
	}

	s, err := c.Sessions.Start(req)
	if err != nil {
		return nil, fmt.Errorf("load realtime session: %w", err)
	}

	return refused(c.Guards.Authenticate(req.WithContext(session.WithSession(ctx, s))))
}

/*
refused turns every failure to authenticate the caller into the
realtime server's refusal, and passes other errors on.
*/
func refused(p fauth.Principal, err error) (fauth.Principal, error) {
	if err == nil {
		return p, nil
	}

	_, known := errors.AsType[*fault.Error](err)
	if known || errors.Is(err, fauth.ErrNoCredentials) {
		return nil, fbroadcasting.ErrUnauthenticated
	}

	return nil, err
}
