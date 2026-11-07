/*
|--------------------------------------------------------------------------
| Widget Session Controller
|--------------------------------------------------------------------------
|
| The handshake a visitor's messenger runs on every page load: start a
| session, resume one, or end it. Starting answers with the token once;
| resuming and ending name the session by its bearer token alone.
|
*/

package widget

import (
	"context"

	"github.com/dimmihq/dimmi/framework/clock"
	fhttp "github.com/dimmihq/dimmi/framework/http"

	actions "github.com/dimmihq/dimmi/app/actions/widget"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
)

/*
SessionOutput is a visitor's session.
*/
type SessionOutput struct {
	Status int
	Body   fhttp.Resource[resources.WidgetSession]
}

/*
SessionController serves visitors' sessions. BaseURL returns the base
address a request reached this install on.
*/
type SessionController struct {
	Start    *actions.StartWidgetSession
	Describe *actions.ShowWidgetSession
	Revoke   *actions.RevokeWidgetSession
	Clock    clock.Clock
	BaseURL  func(ctx context.Context) string
}

/*
Store starts a session and answers with its token.
*/
func (c *SessionController) Store(
	ctx context.Context, in *requests.StoreWidgetSessionRequest,
) (*SessionOutput, error) {
	resolved, _ := actions.CurrentResolved(ctx)
	r, _ := fhttp.RequestFrom(ctx)

	b := in.Body
	start := actions.StartWidgetSessionInput{
		Resolved: resolved, Pages: b.WidgetPages(), Locale: b.Locale, Timezone: b.Timezone, Context: b.Context,
		Honeypot: b.Website, Visitor: visitorOf(r),
	}

	if b.UserJWT != nil {
		start.UserJWT = *b.UserJWT
	}

	if b.PreviousToken != nil {
		start.PreviousToken = *b.PreviousToken
	}

	if b.Page != nil {
		page := b.Page.Page()
		start.Page = &page
	}

	if b.SDK != nil {
		start.SDK = b.SDK.Name
	}

	started, err := c.Start.Handle(ctx, start)
	if err != nil {
		return nil, err
	}

	view := actions.SessionView{Session: started.Session, Contact: started.Contact}
	if started.Session.Kind == string(enums.WidgetSessionIdentified) {
		auth := actions.Authenticated{Session: started.Session, Resolved: resolved}
		if view, err = c.Describe.Describe(ctx, auth); err != nil {
			return nil, err
		}
	}

	if started.Reused {
		return c.answer(ctx, fhttp.StatusOK, view, ""), nil
	}

	return c.answer(ctx, fhttp.StatusCreated, view, started.Token), nil
}

/*
Update proves a signed-in session's customer again with a fresh identity
token and extends it.
*/
func (c *SessionController) Update(
	ctx context.Context, in *requests.UpdateWidgetSessionRequest,
) (*SessionOutput, error) {
	auth := authenticated(ctx)

	refreshed, err := c.Start.Refresh(ctx, auth, in.Body.UserJWT)
	if err != nil {
		return nil, err
	}

	view, err := c.Describe.Describe(ctx, actions.Authenticated{Session: refreshed.Session, Resolved: auth.Resolved})
	if err != nil {
		return nil, err
	}

	return c.answer(ctx, fhttp.StatusOK, view, ""), nil
}

/*
Show resumes an anonymous session.
*/
func (c *SessionController) Show(ctx context.Context, _ *requests.NoInput) (*SessionOutput, error) {
	view, err := c.Describe.Resume(ctx, authenticated(ctx))
	if err != nil {
		return nil, err
	}

	return c.answer(ctx, fhttp.StatusOK, view, ""), nil
}

/*
Destroy ends the session. Ending one already ended answers the same.
*/
func (c *SessionController) Destroy(ctx context.Context, _ *requests.NoInput) (*struct{}, error) {
	auth := authenticated(ctx)

	err := c.Revoke.Handle(ctx, actions.RevokeWidgetSessionInput{
		Session: auth.Session, Reason: enums.WidgetRevokeLogout,
	})
	if err != nil {
		return nil, err
	}

	return &struct{}{}, nil
}

/*
answer wraps the session view in its resource.
*/
func (c *SessionController) answer(
	ctx context.Context, status int, view actions.SessionView, token string,
) *SessionOutput {
	resolved, _ := actions.CurrentResolved(ctx)

	return &SessionOutput{
		Status: status,
		Body: fhttp.NewResource(resources.NewWidgetSession(
			view, resolved.Widget, resources.SessionAnswer{Base: c.BaseURL(ctx), Token: token, Now: c.Clock.Now()},
		)),
	}
}

/*
authenticated returns the request's session with its widget.
*/
func authenticated(ctx context.Context) actions.Authenticated {
	session, _ := actions.CurrentSession(ctx)
	resolved, _ := actions.CurrentResolved(ctx)

	return actions.Authenticated{Session: session, Resolved: resolved}
}

/*
visitorOf returns where the request came from.
*/
func visitorOf(r *fhttp.Request) actions.Visitor {
	if r == nil {
		return actions.Visitor{}
	}

	return actions.Visitor{Origin: r.Header.Get("Origin"), UserAgent: r.UserAgent(), IP: fhttp.ClientIP(r)}
}
