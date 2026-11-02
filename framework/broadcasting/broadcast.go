/*
|--------------------------------------------------------------------------
| Broadcasting
|--------------------------------------------------------------------------
|
| The server publishes, clients listen. Every broadcast is one JSON
| envelope naming the event, stamped with the server's time so clients
| with a wrong clock still show correct relative times, and capped in
| size: listeners are many and slow ones must not fall behind, so a
| large change is announced small and fetched by whoever needs it.
|
| Broadcasts keep the history their channel asks for. Ephemeral ones,
| such as typing indicators, keep none: replaying them after a reconnect
| would show something that is long over.
|
*/

package broadcasting

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/centrifugal/centrifuge"
)

/*
Broadcasting errors. ErrUnknownChannel refuses a channel that no
registered pattern matches. ErrPayloadTooLarge refuses a broadcast whose
encoded envelope exceeds the configured cap; broadcast a reference
instead and let clients fetch the rest.
*/
var (
	ErrUnknownChannel  = errors.New("broadcasting: unknown channel")
	ErrPayloadTooLarge = errors.New("broadcasting: payload too large")
)

/*
Broadcaster publishes events to channels.
*/
type Broadcaster interface {
	/*
		Broadcast publishes the event and its payload on the channel.
	*/
	Broadcast(ctx context.Context, channel, event string, payload any) error

	/*
		BroadcastMany publishes one event on several channels.
	*/
	BroadcastMany(ctx context.Context, channels []string, event string, payload any) error

	/*
		BroadcastEphemeral publishes the event on the channel without
		keeping it in history.
	*/
	BroadcastEphemeral(ctx context.Context, channel, event string, payload any) error
}

/*
Member is one connection listening on a channel that tracks presence.
Info is the public connection info the resolver gave, as JSON.
*/
type Member struct {
	ClientID string
	UserID   string
	Info     json.RawMessage
}

/*
envelope is the shape of every broadcast clients receive.
*/
type envelope struct {
	Event      string    `json:"event"`
	ServerTime time.Time `json:"server_time"`
	Data       any       `json:"data"`
}

/*
revoked closes the connections of a user whose access ended. Its code
is in the range client libraries treat as final, so they do not
reconnect.
*/
func revoked() centrifuge.Disconnect {
	return centrifuge.Disconnect{Code: 4009, Reason: "member_removed"}
}

/*
Broadcast publishes the event and its payload on the channel, keeping
it in the channel's history.
*/
func (s *Server) Broadcast(ctx context.Context, channel, event string, payload any) error {
	return s.BroadcastMany(ctx, []string{channel}, event, payload)
}

/*
BroadcastMany publishes one event on several channels, each keeping it
in its own history. No channel receives it when one is unknown or the
payload is too large.
*/
func (s *Server) BroadcastMany(ctx context.Context, channels []string, event string, payload any) error {
	data, err := s.encode(event, payload)
	if err != nil {
		return err
	}

	routes := make([]route, 0, len(channels))
	for _, channel := range channels {
		r, _, ok := s.channels.match(channel)
		if !ok {
			return fmt.Errorf("broadcast %s on %q: %w", event, channel, ErrUnknownChannel)
		}

		routes = append(routes, r)
	}

	for i, channel := range channels {
		if err := s.publish(ctx, channel, data, routes[i].opts.history()...); err != nil {
			return fmt.Errorf("broadcast %s: %w", event, err)
		}
	}

	return nil
}

/*
BroadcastEphemeral publishes the event on the channel without keeping
it in history, so reconnecting clients never receive it.
*/
func (s *Server) BroadcastEphemeral(ctx context.Context, channel, event string, payload any) error {
	data, err := s.encode(event, payload)
	if err != nil {
		return err
	}

	if _, _, ok := s.channels.match(channel); !ok {
		return fmt.Errorf("broadcast %s on %q: %w", event, channel, ErrUnknownChannel)
	}

	if err := s.publish(ctx, channel, data); err != nil {
		return fmt.Errorf("broadcast %s: %w", event, err)
	}

	return nil
}

/*
Disconnect closes every connection of the user on every node and tells
the clients not to reconnect. Use it when access ends: the user is
removed, or a session or token is revoked.
*/
func (s *Server) Disconnect(ctx context.Context, userID string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("disconnect user %s: %w", userID, err)
	}

	disconnect := revoked()
	if err := s.node.Disconnect(userID, centrifuge.WithCustomDisconnect(disconnect)); err != nil {
		return fmt.Errorf("disconnect user %s: %w", userID, err)
	}

	return nil
}

/*
Presence returns the connections listening on the channel, across every
node, ordered by user and then client.
*/
func (s *Server) Presence(ctx context.Context, channel string) ([]Member, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("presence on %q: %w", channel, err)
	}

	result, err := s.node.Presence(channel)
	if err != nil {
		return nil, fmt.Errorf("presence on %q: %w", channel, err)
	}

	members := make([]Member, 0, len(result.Presence))
	for _, info := range result.Presence {
		members = append(members, Member{ClientID: info.ClientID, UserID: info.UserID, Info: info.ConnInfo})
	}

	slices.SortFunc(members, func(a, b Member) int {
		return cmp.Or(strings.Compare(a.UserID, b.UserID), strings.Compare(a.ClientID, b.ClientID))
	})

	return members, nil
}

/*
encode builds the envelope for the event and checks its size.
*/
func (s *Server) encode(event string, payload any) ([]byte, error) {
	data, err := json.Marshal(envelope{Event: event, ServerTime: s.clock.Now().UTC(), Data: payload})
	if err != nil {
		return nil, fmt.Errorf("encode %s: %w", event, err)
	}

	if len(data) > s.maxPayload {
		return nil, fmt.Errorf("broadcast %s: %w: %d bytes, cap %d", event, ErrPayloadTooLarge, len(data), s.maxPayload)
	}

	return data, nil
}

/*
publish hands one encoded broadcast to the broker.
*/
func (s *Server) publish(ctx context.Context, channel string, data []byte, opts ...centrifuge.PublishOption) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("publish on %q: %w", channel, err)
	}

	if _, err := s.node.Publish(channel, data, opts...); err != nil {
		return fmt.Errorf("publish on %q: %w", channel, err)
	}

	return nil
}
