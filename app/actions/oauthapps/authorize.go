/*
|--------------------------------------------------------------------------
| Connecting An App
|--------------------------------------------------------------------------
|
| An app asks to act for a person: which app it is, where it sends the
| answer and what it wants to do. The person sees the request, picks one
| of their workspaces and the access to give, and approves or refuses.
| The request is checked again when the answer comes back rather than
| trusted from the browser, the workspace must still be one the person
| is an active member of, and the access given is never more than the
| app asked for. Whatever is given, the person's role still limits what
| the app can do, request by request.
|
*/

package oauthapps

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/oauth"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Request is an app's request as the person reviews it: the app, whether
it registered itself so dimmi cannot vouch for it, the address it sends
the answer to and whether that is on the person's own computer, the
access it asks for and the workspaces the person may connect it to.
*/
type Request struct {
	ClientID     string
	ClientName   string
	SelfMade     bool
	RedirectHost string
	Loopback     bool
	Scopes       []string
	Workspaces   []models.ListUserActiveMembershipsRow
}

/*
Decision is the person's answer: the request's query as the app sent
it, whether they approved, the workspace and the access they gave, and
when they last signed in fully.
*/
type Decision struct {
	UserID    ids.UUID
	Query     url.Values
	Approve   bool
	Workspace ids.UUID
	Scopes    []string
	AuthTime  time.Time
}

/*
ErrNoWorkspace refuses an approval for a workspace the person is not an
active member of.
*/
var ErrNoWorkspace = fault.New(fault.Invalid, "workspace_invalid",
	"Choose one of your workspaces to connect the app to.").OnField("workspace_id")

/*
ErrNoScopes refuses an approval that gives no access at all.
*/
var ErrNoScopes = fault.New(fault.Invalid, "scopes_required",
	"Give the app at least one kind of access, or deny the request.").OnField("scopes")

/*
Authorize reviews and decides app requests for the signed-in person.

Rules:
  - The request is checked against the OAuth server on every call.
  - Only the person's active workspaces, outside the trash, are offered.
  - Access given is a subset of what the app asked for.

Errors: an [oauth.Error] for a request that is not valid, with a redirect
when the app may hear of it, [ErrNoWorkspace], [ErrNoScopes], or a
wrapped database error.
*/
type Authorize struct {
	DB     *database.DB
	Server *oauth.Server
}

/*
Review checks a request and describes it for the person.
*/
func (a *Authorize) Review(ctx context.Context, user ids.UUID, query url.Values) (Request, error) {
	req, err := a.Server.ValidateAuthorization(ctx, query)
	if err != nil {
		return Request{}, err
	}

	out := Request{
		ClientID: req.Client.ID, ClientName: req.Client.Name, SelfMade: req.Client.Dynamic,
		Scopes: offered(req.Scopes),
	}

	if u, err := url.Parse(req.RedirectURI); err == nil {
		out.RedirectHost = u.Host
		out.Loopback = loopback(u.Hostname())
	}

	out.Workspaces, err = a.workspaces(ctx, user)

	return out, err
}

/*
Decide answers a request and returns where to send the person: back to
the app with a code, or with the news that they refused.
*/
func (a *Authorize) Decide(ctx context.Context, d Decision) (string, error) {
	req, err := a.Server.ValidateAuthorization(ctx, d.Query)
	if err != nil {
		return "", err
	}

	if !d.Approve {
		return a.Server.Deny(req), nil
	}

	scopes := chosen(req.Scopes, d.Scopes)
	if len(scopes) == 0 {
		return "", ErrNoScopes
	}

	workspaces, err := a.workspaces(ctx, d.UserID)
	if err != nil {
		return "", err
	}

	if !slices.ContainsFunc(workspaces, func(w models.ListUserActiveMembershipsRow) bool {
		return w.WorkspaceID == d.Workspace
	}) {
		return "", ErrNoWorkspace
	}

	to, err := a.Server.Approve(ctx, req, oauth.Approval{
		User: d.UserID, Tenant: d.Workspace.String(), Scopes: scopes, AuthTime: d.AuthTime,
	})
	if err != nil {
		return "", fmt.Errorf("approve app: %w", err)
	}

	return to, nil
}

/*
workspaces lists the person's active workspaces outside the trash.
*/
func (a *Authorize) workspaces(ctx context.Context, user ids.UUID) ([]models.ListUserActiveMembershipsRow, error) {
	rows, err := models.New(a.DB).ListUserActiveMemberships(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("list workspaces: %w", err)
	}

	return slices.DeleteFunc(rows, func(r models.ListUserActiveMembershipsRow) bool { return r.Trashed }), nil
}

/*
offered is the access a request asks for, as the person chooses from:
offline access says nothing to a person, so it is never shown.
*/
func offered(scopes []string) []string {
	return slices.DeleteFunc(slices.Clone(scopes), func(s string) bool { return s == appauth.OfflineAccess })
}

/*
chosen is the access to give: what the person picked of what the app
asked for, everything asked when they picked nothing, and offline access
whenever the app asked for it.
*/
func chosen(asked, picked []string) []string {
	if picked == nil {
		return slices.Clone(asked)
	}

	var out []string
	for _, scope := range picked {
		if slices.Contains(asked, scope) && !slices.Contains(out, scope) {
			out = append(out, scope)
		}
	}

	if len(out) > 0 && slices.Contains(asked, appauth.OfflineAccess) && !slices.Contains(out, appauth.OfflineAccess) {
		out = append(out, appauth.OfflineAccess)
	}

	return out
}

/*
loopback reports whether a host is the person's own computer.
*/
func loopback(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "::1" || strings.HasSuffix(host, ".localhost")
}

/*
IsRefusal reports whether an error is the OAuth server refusing the
request, and returns it.
*/
func IsRefusal(err error) (*oauth.Error, bool) {
	return errors.AsType[*oauth.Error](err)
}
