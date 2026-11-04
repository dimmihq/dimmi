/*
|--------------------------------------------------------------------------
| Connecting And Subscribing
|--------------------------------------------------------------------------
|
| A connection is authenticated once, when it opens, by the resolver the
| application hands over: the same guards that protect its HTTP routes
| look at the upgrade request and at the token the client sends in its
| connect command, never in the URL of a bidirectional connection. Every
| subscription is then checked by the authorizer of the channel's
| pattern, with the principal the resolver returned.
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
*/
type Principal struct {
	UserID   string
	Info     any
	Channels []string
	Subject  any
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

	return centrifuge.ConnectReply{
		Context:       ctx,
		Credentials:   &centrifuge.Credentials{UserID: principal.UserID, Info: info},
		Subscriptions: subscriptions,
	}, nil
}

/*
connected guards the subscriptions a client asks for, and lets it list
who else listens on a channel it is subscribed to. Publishing and every
other client command stay unhandled, which the node refuses.
*/
func (s *Server) connected(client *centrifuge.Client) {
	client.OnSubscribe(func(event centrifuge.SubscribeEvent, callback centrifuge.SubscribeCallback) {
		opts, err := s.authorize(client.Context(), event.Channel)
		callback(centrifuge.SubscribeReply{Options: opts}, err)
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
