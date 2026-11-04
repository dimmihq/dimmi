/*
|--------------------------------------------------------------------------
| Connecting And Subscribing
|--------------------------------------------------------------------------
|
| A connection is authenticated when it opens by the resolver the
| application hands over: the same guards that protect its HTTP routes
| look at the upgrade request and at the token the client sends in its
| connect command, never in the URL of a bidirectional connection. Every
| subscription is then checked by the authorizer of the channel's
| pattern, with the principal the resolver returned.
|
| A socket must not outlive the credential that opened it. The application
| closes connections when it ends a credential, but a credential can also
| just run out, or a closing message can be lost, so every connection also
| asks whether its credential is still alive: before each subscription, and
| on a timer. A dead one is closed with a code clients reconnect after, and
| only a credential that still works gets back in, such as a browser whose
| session cookie moved on to a new session.
|
| Clients never publish. Everything on a channel comes from the server,
| so a client cannot impersonate another or flood its listeners.
|
*/

package broadcasting

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/centrifugal/centrifuge"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/log"
)

/*
ErrUnauthenticated is what a resolver returns, wrapped or not, to refuse
a connection whose credentials are missing, invalid or expired. Any
other error is logged and refused as a server error.
*/
var ErrUnauthenticated = errors.New("broadcasting: unauthenticated")

/*
Principal is who is connecting, as the resolver decided.

UserID names the user; Disconnect closes every connection of a user, on
every node. Info is public information about the connection, encoded as
JSON into presence and join and leave notices. Channels lists channels
the server subscribes the connection to on its own, which is how
clients that cannot send commands, such as EventSource, listen; each
still passes its authorizer. Subject is the application's own identity
value, handed back untouched to authorizers.

Credential names what the connection authenticated with, which
DisconnectCredential closes it by. Alive reports whether that credential
still works; left nil, the connection is never asked. An error is not an
answer: the connection stays until it can be asked again, but takes no
new subscription.
*/
type Principal struct {
	UserID     string
	Info       any
	Channels   []string
	Subject    any
	Credential string
	Alive      func(ctx context.Context) (bool, error)
}

/*
Resolver authenticates a connection from the HTTP request that opened
it and the token the client sent, which is empty when it sent none.
*/
type Resolver func(ctx context.Context, r *http.Request, token string) (Principal, error)

type (
	requestKey   struct{}
	principalKey struct{}
)

/*
withRequest makes the opening request available to the resolver, which
runs deep inside the connection handling.
*/
func withRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestKey{}, r)))
	})
}

/*
connecting authenticates a new connection and subscribes it to the
channels its principal lists.
*/
func (s *Server) connecting(ctx context.Context, event centrifuge.ConnectEvent) (centrifuge.ConnectReply, error) {
	r, ok := ctx.Value(requestKey{}).(*http.Request)
	if !ok {
		return centrifuge.ConnectReply{}, centrifuge.ErrorUnauthorized
	}

	principal, err := s.resolve(ctx, r, event.Token)
	if errors.Is(err, ErrUnauthenticated) {
		return centrifuge.ConnectReply{}, centrifuge.ErrorUnauthorized
	}

	if err != nil {
		s.logger.ErrorContext(ctx, "resolve realtime connection", log.Err(err))

		return centrifuge.ConnectReply{}, centrifuge.ErrorInternal
	}

	info, err := encodeInfo(principal.Info)
	if err != nil {
		s.logger.ErrorContext(ctx, "encode realtime connection info", log.Err(err))

		return centrifuge.ConnectReply{}, centrifuge.ErrorInternal
	}

	ctx = context.WithValue(ctx, principalKey{}, principal)

	subscriptions := make(map[string]centrifuge.SubscribeOptions, len(principal.Channels))
	for _, channel := range principal.Channels {
		opts, err := s.authorize(ctx, channel)
		if err != nil {
			return centrifuge.ConnectReply{}, err
		}

		subscriptions[channel] = opts
	}

	credentials := &centrifuge.Credentials{UserID: principal.UserID, Info: info}
	if principal.Alive != nil {
		credentials.ExpireAt = s.nextRecheck()
	}

	return centrifuge.ConnectReply{Context: ctx, Credentials: credentials, Subscriptions: subscriptions}, nil
}

/*
connected guards the subscriptions a client asks for, lets it list who
else listens on a channel it is subscribed to, keeps asking whether its
credential is alive and reports who comes and goes. Publishing and every
other client command stay unhandled, which the node refuses.
*/
func (s *Server) connected(client *centrifuge.Client) {
	client.OnSubscribe(func(event centrifuge.SubscribeEvent, callback centrifuge.SubscribeCallback) {
		if err := s.checkAlive(client.Context()); err != nil {
			callback(centrifuge.SubscribeReply{}, err)

			return
		}

		opts, err := s.authorize(client.Context(), event.Channel)
		callback(centrifuge.SubscribeReply{Options: opts}, err)

		if err == nil {
			s.listenersChanged(client, event.Channel)
		}
	})
	client.OnUnsubscribe(func(event centrifuge.UnsubscribeEvent) {
		s.listenersChanged(client, event.Channel)
	})
	client.OnRefresh(func(_ centrifuge.RefreshEvent, callback centrifuge.RefreshCallback) {
		err := s.checkAlive(client.Context())
		if errors.Is(err, centrifuge.DisconnectExpired) {
			callback(centrifuge.RefreshReply{Expired: true}, nil)

			return
		}

		callback(centrifuge.RefreshReply{ExpireAt: s.nextRecheck()}, nil)
	})
	client.OnPresence(func(event centrifuge.PresenceEvent, callback centrifuge.PresenceCallback) {
		if !client.IsSubscribed(event.Channel) {
			callback(centrifuge.PresenceReply{}, centrifuge.ErrorPermissionDenied)

			return
		}

		callback(centrifuge.PresenceReply{}, nil)
	})
}

/*
listenersChanged tells the channel's pattern, when it asked, that the
connection started or stopped listening.
*/
func (s *Server) listenersChanged(client *centrifuge.Client, channel string) {
	r, _, ok := s.channels.match(channel)
	if !ok || r.opts.ListenersChanged == nil {
		return
	}

	principal, _ := client.Context().Value(principalKey{}).(Principal)
	r.opts.ListenersChanged(client.Context(), principal, channel)
}

/*
checkAlive returns nil while the connection's credential works, the
disconnect that ends the connection once it does not, and a server error
when the answer could not be had.
*/
func (s *Server) checkAlive(ctx context.Context) error {
	principal, _ := ctx.Value(principalKey{}).(Principal)
	if principal.Alive == nil {
		return nil
	}

	alive, err := principal.Alive(ctx)
	if err != nil {
		s.logger.ErrorContext(ctx, "check realtime credential", log.Err(err))

		return centrifuge.ErrorInternal
	}

	if !alive {
		return centrifuge.DisconnectExpired
	}

	return nil
}

/*
nextRecheck returns when a connection is next asked about its
credential, as the Unix time the node compares with the wall clock.
*/
func (s *Server) nextRecheck() int64 {
	return clock.System().Now().Add(s.recheck).Unix()
}

/*
authorize runs the authorizer of the channel's pattern for the
connection's principal and returns the channel's subscription options.
*/
func (s *Server) authorize(ctx context.Context, channel string) (centrifuge.SubscribeOptions, error) {
	r, params, ok := s.channels.match(channel)
	if !ok {
		return centrifuge.SubscribeOptions{}, centrifuge.ErrorUnknownChannel
	}

	principal, _ := ctx.Value(principalKey{}).(Principal)

	allowed, err := r.authorize(ctx, principal, params)
	if err != nil {
		s.logger.ErrorContext(ctx, "authorize realtime channel", log.String("channel", channel), log.Err(err))

		return centrifuge.SubscribeOptions{}, centrifuge.ErrorInternal
	}

	if !allowed {
		return centrifuge.SubscribeOptions{}, centrifuge.ErrorPermissionDenied
	}

	return r.opts.subscribe(), nil
}

/*
encodeInfo encodes the public connection info, leaving it empty when
there is none.
*/
func encodeInfo(info any) ([]byte, error) {
	if info == nil {
		return nil, nil
	}

	return json.Marshal(info)
}
