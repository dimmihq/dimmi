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
| A broadcast may carry an id, so a client that receives it twice, once
| replayed from history and once live, keeps it once, and metadata such
| as the id of the client change that caused it, so the client that made
| the change can recognize its own echo.
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
Message is one broadcast. ID identifies it to clients that deduplicate,
and may be empty. Event names it and Data is its payload, encoded as
JSON. Meta carries facts about the broadcast itself rather than its
subject, and is left out when empty. Ephemeral keeps the broadcast out
of every channel's history.
*/
type Message struct {
	ID        string
	Event     string
	Data      any
	Meta      map[string]string
	Ephemeral bool
}

/*
envelope is the shape of every broadcast clients receive.
*/
type envelope struct {
	ID         string            `json:"id,omitempty"`
	Event      string            `json:"event"`
	ServerTime time.Time         `json:"server_time"`
	Data       any               `json:"data"`
	Meta       map[string]string `json:"meta,omitempty"`
}

/*
revoked closes the connections of a user whose access ended. Client
libraries reconnect after a code in this range, and the new connection
is then refused outright or refused every channel the user may no longer
follow, so a client knows to drop what it showed.
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
	return s.Publish(ctx, channels, Message{Event: event, Data: payload})
}

/*
BroadcastEphemeral publishes the event on the channel without keeping
it in history, so reconnecting clients never receive it.
*/
func (s *Server) BroadcastEphemeral(ctx context.Context, channel, event string, payload any) error {
	return s.Publish(ctx, []string{channel}, Message{Event: event, Data: payload, Ephemeral: true})
}

/*
Publish publishes the message on every channel, each keeping it in its
own history unless the message is ephemeral. No channel receives it
when one is unknown or the encoded message is too large.
*/
func (s *Server) Publish(ctx context.Context, channels []string, m Message) error {
	data, err := s.encode(m)
	if err != nil {
		return err
	}

	routes := make([]route, 0, len(channels))
	for _, channel := range channels {
		r, _, ok := s.channels.match(channel)
		if !ok {
			return fmt.Errorf("broadcast %s on %q: %w", m.Event, channel, ErrUnknownChannel)
		}

		routes = append(routes, r)
	}

	for i, channel := range channels {
		var opts []centrifuge.PublishOption
		if !m.Ephemeral {
			opts = routes[i].opts.history()
		}

		if err := s.send(ctx, channel, data, opts...); err != nil {
			return fmt.Errorf("broadcast %s: %w", m.Event, err)
		}
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
Unsubscribe takes every connection of the user off the channel, on every
node, leaving their other subscriptions and the connections themselves
alone. Use it when someone stops being allowed on one channel.
*/
func (s *Server) Unsubscribe(ctx context.Context, userID, channel string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("unsubscribe user %s from %s: %w", userID, channel, err)
	}

	if err := s.node.Unsubscribe(userID, channel); err != nil {
		return fmt.Errorf("unsubscribe user %s from %s: %w", userID, channel, err)
	}

	return nil
}

/*
credentialEnded is the notice that tells every node to close the
connections a user opened with one credential.
*/
type credentialEnded struct {
	UserID     string `json:"user_id"`
	Credential string `json:"credential"`
}

/*
opCloseConnections names that notice among the messages nodes send each
other.
*/
const opCloseConnections = "close_connections"

/*
DisconnectCredential closes the user's connections opened with the
credential, on every node, leaving those opened with any other. Use it
when one session or token ends and the user's others go on.
*/
func (s *Server) DisconnectCredential(ctx context.Context, userID, credential string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("disconnect credential of user %s: %w", userID, err)
	}

	data, err := json.Marshal(credentialEnded{UserID: userID, Credential: credential})
	if err != nil {
		return fmt.Errorf("encode ended credential: %w", err)
	}

	if err := s.node.Notify(opCloseConnections, data, ""); err != nil {
		return fmt.Errorf("disconnect credential of user %s: %w", userID, err)
	}

	return nil
}

/*
noticed handles what another node, or this one, tells every node: the
connections of an ended credential close here. Anything it cannot read
came from a node of another version, and is left to the timed check.
*/
func (s *Server) noticed(event centrifuge.NotificationEvent) {
	ended := credentialEnded{}
	if event.Op != opCloseConnections || json.Unmarshal(event.Data, &ended) != nil {
		return
	}

	for _, client := range s.node.Hub().UserConnections(ended.UserID) {
		principal, _ := client.Context().Value(principalKey{}).(Principal)
		if principal.Credential == ended.Credential {
			client.Disconnect(revoked())
		}
	}
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
encode builds the envelope for the message and checks its size.
*/
func (s *Server) encode(m Message) ([]byte, error) {
	data, err := json.Marshal(envelope{
		ID: m.ID, Event: m.Event, ServerTime: s.clock.Now().UTC(), Data: m.Data, Meta: m.Meta,
	})
	if err != nil {
		return nil, fmt.Errorf("encode %s: %w", m.Event, err)
	}

	if len(data) > s.maxPayload {
		return nil, fmt.Errorf("broadcast %s: %w: %d bytes, cap %d",
			m.Event, ErrPayloadTooLarge, len(data), s.maxPayload)
	}

	return data, nil
}

/*
send hands one encoded broadcast to the broker.
*/
func (s *Server) send(ctx context.Context, channel string, data []byte, opts ...centrifuge.PublishOption) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("publish on %q: %w", channel, err)
	}

	if _, err := s.node.Publish(channel, data, opts...); err != nil {
		return fmt.Errorf("publish on %q: %w", channel, err)
	}

	return nil
}
