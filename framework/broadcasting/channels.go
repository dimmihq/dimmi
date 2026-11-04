/*
|--------------------------------------------------------------------------
| Channels
|--------------------------------------------------------------------------
|
| Every channel is private. A channel exists only when a registered
| pattern matches its name, and the pattern's authorizer decides who may
| listen. Unknown channels are refused for subscribing and broadcasting
| alike, so a typo can never open a channel nobody guards.
|
| A pattern also carries the options of every channel it matches:
| whether the channel tracks who is listening, whether it announces
| arrivals and departures, and how much history it keeps for clients
| that reconnect.
|
*/

package broadcasting

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/centrifugal/centrifuge"
)

/*
ErrInvalidPattern rejects a channel pattern with an empty, repeated or
unbalanced placeholder.
*/
var ErrInvalidPattern = errors.New("broadcasting: invalid channel pattern")

/*
Authorizer decides whether the principal may listen on a channel. Params
holds the values of the pattern's placeholders, so the pattern
"room:{id}" hands over params["id"] for the channel "room:42".
*/
type Authorizer func(ctx context.Context, principal Principal, params map[string]string) (bool, error)

/*
ChannelOptions configure every channel a pattern matches.

Presence keeps the list of connected listeners, read with Presence.
JoinLeave also tells listeners when someone arrives or leaves.
HistorySize and HistoryTTL keep that many recent broadcasts for that
long, so a client that reconnects in time receives what it missed; both
must be positive for history to be kept.

ListenersChanged runs each time a connection starts or stops listening
on a channel of the pattern, with who it is and the channel's name. It
runs on the connection's own goroutine, so it hands long work elsewhere,
and just before the change shows in Presence, so it reads the listeners
a moment later.
*/
type ChannelOptions struct {
	Presence         bool
	JoinLeave        bool
	HistorySize      int
	HistoryTTL       time.Duration
	ListenersChanged func(principal Principal, channel string)
}

/*
Channels holds the registered channel patterns. It is safe for
concurrent use.
*/
type Channels struct {
	mu     sync.RWMutex
	routes []route
}

type route struct {
	expr      *regexp.Regexp
	names     []string
	opts      ChannelOptions
	authorize Authorizer
}

/*
Authorize registers a pattern, the options of its channels and the
authorizer that guards them. Placeholders in braces match one segment of
the name, up to the next colon. The first registered pattern that
matches a channel wins.
*/
func (c *Channels) Authorize(pattern string, opts ChannelOptions, authorize Authorizer) error {
	expr, names, err := compile(pattern)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.routes = append(c.routes, route{expr: expr, names: names, opts: opts, authorize: authorize})

	return nil
}

/*
MustAuthorize is Authorize for patterns written in code, which are
either valid or a programming error: an invalid pattern panics while
the application boots, long before any client could connect.
*/
func (c *Channels) MustAuthorize(pattern string, opts ChannelOptions, authorize Authorizer) {
	if err := c.Authorize(pattern, opts, authorize); err != nil {
		panic(err)
	}
}

/*
match finds the route for a channel and the values of its placeholders.
*/
func (c *Channels) match(channel string) (route, map[string]string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, r := range c.routes {
		values := r.expr.FindStringSubmatch(channel)
		if values == nil {
			continue
		}

		params := make(map[string]string, len(r.names))
		for i, name := range r.names {
			params[name] = values[i+1]
		}

		return r, params, true
	}

	return route{}, nil, false
}

/*
compile turns a pattern into an anchored expression and the names of
its placeholders, in order.
*/
func compile(pattern string) (*regexp.Regexp, []string, error) {
	var expr strings.Builder

	names := []string{}
	rest := pattern

	expr.WriteString("^")

	for {
		literal, after, found := strings.Cut(rest, "{")
		if strings.Contains(literal, "}") {
			return nil, nil, fmt.Errorf("%w %q", ErrInvalidPattern, pattern)
		}

		expr.WriteString(regexp.QuoteMeta(literal))

		if !found {
			break
		}

		name, tail, closed := strings.Cut(after, "}")
		if !closed || name == "" || strings.Contains(name, "{") || slices.Contains(names, name) {
			return nil, nil, fmt.Errorf("%w %q", ErrInvalidPattern, pattern)
		}

		names = append(names, name)
		expr.WriteString("([^:]+)")
		rest = tail
	}

	expr.WriteString("$")

	return regexp.MustCompile(expr.String()), names, nil
}

/*
history returns the publish options that keep a broadcast for clients
that reconnect, or none when the channel keeps no history.
*/
func (o ChannelOptions) history() []centrifuge.PublishOption {
	if !o.recoverable() {
		return nil
	}

	return []centrifuge.PublishOption{centrifuge.WithHistory(o.HistorySize, o.HistoryTTL)}
}

/*
subscribe returns the subscription options for a listener.
*/
func (o ChannelOptions) subscribe() centrifuge.SubscribeOptions {
	return centrifuge.SubscribeOptions{
		EmitPresence:      o.Presence,
		EmitJoinLeave:     o.JoinLeave,
		PushJoinLeave:     o.JoinLeave,
		EnablePositioning: o.recoverable(),
		EnableRecovery:    o.recoverable(),
	}
}

func (o ChannelOptions) recoverable() bool {
	return o.HistorySize > 0 && o.HistoryTTL > 0
}
