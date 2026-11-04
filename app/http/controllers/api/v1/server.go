/*
|--------------------------------------------------------------------------
| Controller Support
|--------------------------------------------------------------------------
|
| What every controller of the API needs from the request beyond its
| input: who is calling, from which client, and facts about this server
| that responses carry. Keeping them here keeps each controller to
| reading its input, calling its action and shaping the answer.
|
*/

package v1

import (
	"context"

	"github.com/dimmihq/dimmi/framework/clock"
	fhttp "github.com/dimmihq/dimmi/framework/http"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/http/resources"
)

/*
Server is what responses say about this server. Version is the release,
APIVersion the API dialect, MailConfigured whether mail actually
leaves the server, and AuditRetentionDays how long audit entries are
kept, 0 for ever.
*/
type Server struct {
	Edition            string
	Version            string
	APIVersion         string
	Region             string
	MailConfigured     bool
	AuditRetentionDays int
	Clock              clock.Clock
}

/*
instance returns the server facts carried by a response.
*/
func (s Server) instance() resources.Instance {
	return resources.Instance{
		Edition:        s.Edition,
		Version:        s.Version,
		APIVersion:     s.APIVersion,
		Region:         s.Region,
		MailConfigured: s.MailConfigured,
		AuditRetention: s.AuditRetentionDays,
		ServerTime:     s.Clock.Now().UTC(),
	}
}

/*
MeOutput answers with the caller's picture, and hands the browser a
device cookie when it had none.
*/
type MeOutput struct {
	SetCookie []fhttp.Cookie `header:"Set-Cookie"`
	Body      fhttp.Resource[resources.Me]
}

/*
EmptyOutput is the answer of an endpoint that returns no body.
*/
type EmptyOutput struct{}

/*
clientOf returns the client the request came from. The router puts the
request in the context of every typed route.
*/
func clientOf(ctx context.Context, cookies appauth.Cookies) appauth.Client {
	r, _ := fhttp.RequestFrom(ctx)

	return appauth.Client{IP: fhttp.ClientIP(r), UserAgent: r.UserAgent(), DeviceToken: cookies.DeviceToken(r)}
}

/*
caller returns the principal the guard stored. Only routes behind the
auth middleware call it, and that middleware stores one or refuses the
request.
*/
func caller(ctx context.Context) *appauth.Principal {
	p, _ := appauth.CurrentPrincipal(ctx)

	return p
}

/*
deviceCookies returns the device cookie to set, none when the browser
already holds one.
*/
func deviceCookies(cookies appauth.Cookies, token string) []fhttp.Cookie {
	if token == "" {
		return nil
	}

	return []fhttp.Cookie{cookies.DeviceCookie(token)}
}
