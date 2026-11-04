/*
|--------------------------------------------------------------------------
| Visitor Request Context
|--------------------------------------------------------------------------
|
| A visitor's request carries the widget it reached and, once authenticated,
| the session it holds. Middleware stores them; controllers and actions read
| them from here, so nothing further down looks them up again.
|
*/

package widget

import (
	"context"

	"github.com/dimmihq/dimmi/app/models"
)

type (
	resolvedKey struct{}
	sessionKey  struct{}
)

/*
WithResolved returns the context carrying the widget.
*/
func WithResolved(ctx context.Context, r Resolved) context.Context {
	return context.WithValue(ctx, resolvedKey{}, r)
}

/*
CurrentResolved returns the widget of the request.
*/
func CurrentResolved(ctx context.Context) (Resolved, bool) {
	r, ok := ctx.Value(resolvedKey{}).(Resolved)

	return r, ok
}

/*
WithSession returns the context carrying the visitor's session.
*/
func WithSession(ctx context.Context, s models.WidgetSession) context.Context {
	return context.WithValue(ctx, sessionKey{}, s)
}

/*
CurrentSession returns the visitor's session.
*/
func CurrentSession(ctx context.Context) (models.WidgetSession, bool) {
	s, ok := ctx.Value(sessionKey{}).(models.WidgetSession)

	return s, ok
}
