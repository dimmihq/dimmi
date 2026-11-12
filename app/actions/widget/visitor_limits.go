/*
|--------------------------------------------------------------------------
| Visitor Limits
|--------------------------------------------------------------------------
|
| The messenger is open to anyone who loads a page, so a script can fill a
| team's inbox or its storage in minutes. Each visitor session may send
| only so many messages, start only so many conversations and upload only
| so many files in a window, and so may all the sessions from one network.
| A busy office shares one network, so a network's limits are far wider
| than a session's. A visitor over a limit is told to wait, with how long.
|
*/

package widget

import (
	"context"
	"time"

	"github.com/dimmihq/dimmi/framework/ratelimit"

	appauth "github.com/dimmihq/dimmi/app/auth"
)

/*
The limits: what one session may do in a window, and what every session
of one network may do together.
*/
const (
	messagesPerSession      = 20
	messagesPerNetwork      = 600
	uploadsPerSession       = 10
	uploadsPerNetwork       = 300
	conversationsPerSession = 5
	conversationsPerNetwork = 60
)

/*
VisitorLimits counts what visitors do against their limits. A nil value
limits nothing.
*/
type VisitorLimits struct {
	Limiter *ratelimit.Limiter
}

/*
visitorRate is one kind of action and what it may do: Session is how many
times a session may do it in Window, Network how many times every session
of one network may do it in an hour.
*/
type visitorRate struct {
	action  string
	session int
	network int
	window  time.Duration
}

/*
Message counts a message against the session's minute and the network's
hour.
*/
func (l *VisitorLimits) Message(ctx context.Context, auth Authenticated) error {
	return l.count(ctx, auth, visitorRate{"message", messagesPerSession, messagesPerNetwork, time.Minute})
}

/*
Upload counts an announced file against the session's minute and the
network's hour.
*/
func (l *VisitorLimits) Upload(ctx context.Context, auth Authenticated) error {
	return l.count(ctx, auth, visitorRate{"upload", uploadsPerSession, uploadsPerNetwork, time.Minute})
}

/*
Conversation counts a new conversation against the session's hour and
the network's hour.
*/
func (l *VisitorLimits) Conversation(ctx context.Context, auth Authenticated) error {
	return l.count(
		ctx, auth, visitorRate{"conversation", conversationsPerSession, conversationsPerNetwork, time.Hour},
	)
}

/*
count counts one action against the session's limit and the network's.
*/
func (l *VisitorLimits) count(ctx context.Context, auth Authenticated, rate visitorRate) error {
	if l == nil || l.Limiter == nil {
		return nil
	}

	session := auth.Session
	rates := []appauth.Rate{
		{Key: "widget_" + rate.action + ":session:" + session.ID.String(), Limit: rate.session, Window: rate.window},
	}

	if session.IpPrefix != nil {
		rates = append(rates, appauth.Rate{
			Key: "widget_" + rate.action + ":network:" + session.IpPrefix.String(), Limit: rate.network,
			Window: time.Hour,
		})
	}

	return appauth.Throttle(ctx, l.Limiter, rates...)
}
