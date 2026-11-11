/*
|--------------------------------------------------------------------------
| Realtime Test Client
|--------------------------------------------------------------------------
|
| Realtime tests listen the way real clients do: over a WebSocket to a
| real server on a real port, speaking the same JSON protocol as the
| browser SDK. There is no fake broadcaster to assert on, so a test that
| passes proves a subscriber actually received the broadcast, through
| the authentication, authorization, history and presence a client
| would go through.
|
| Every read waits a bounded time, so a broadcast that never arrives
| fails the test instead of hanging it.
|
*/

package testkit

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/stretchr/testify/require"
)

/*
RealtimeWait bounds every read of a realtime test client.
*/
const RealtimeWait = 10 * time.Second

/*
realtimePath is where realtime servers accept WebSocket clients.
*/
const realtimePath = "/connection/websocket"

/*
Serve serves the handler on a local port until the test ends and
returns its base URL, for clients that need a real socket.
*/
func Serve(tb testing.TB, handler http.Handler) string {
	tb.Helper()

	srv := httptest.NewServer(handler)
	Cleanup(tb, srv.Close)

	return srv.URL
}

/*
RealtimePublication is one broadcast as a client receives it: its
channel, its position in the channel's history, and the envelope the
server wrapped it in.
*/
type RealtimePublication struct {
	Channel string
	Offset  uint64
	ID      string
	Event   string
	Data    json.RawMessage
	Meta    map[string]string
}

/*
RealtimeSubscription is the answer to a subscription: Error when it
was refused, otherwise the channel's position and, when recovering,
the publications missed and whether recovery succeeded.
*/
type RealtimeSubscription struct {
	Error        *RealtimeError
	Recoverable  bool
	Epoch        string
	Offset       uint64
	Recovered    bool
	Publications []RealtimePublication
}

/*
RealtimeOptions open a connection: Header goes with the upgrade
request, such as cookies and Origin, and Token in the connect command.
*/
type RealtimeOptions struct {
	Header http.Header
	Token  string
}

/*
Realtime is a connected realtime test client.
*/
type Realtime struct {
	tb    testing.TB
	conn  *websocket.Conn
	id    int
	queue []realtimeFrame
}

/*
DialRealtime connects to the server at the base URL and sends the
connect command. It returns the client and the server's refusal, nil
when the connection was accepted.
*/
func DialRealtime(tb testing.TB, baseURL string, opts RealtimeOptions) (*Realtime, *RealtimeError) {
	tb.Helper()

	ctx, cancel := context.WithTimeout(tb.Context(), RealtimeWait)
	defer cancel()

	target := "ws" + strings.TrimPrefix(baseURL, "http") + realtimePath

	conn, resp, err := websocket.Dial(ctx, target, &websocket.DialOptions{HTTPHeader: opts.Header})
	if resp != nil && resp.Body != nil {
		require.NoError(tb, resp.Body.Close())
	}

	require.NoError(tb, err)
	Cleanup(tb, func() { _ = conn.CloseNow() })

	c := &Realtime{tb: tb, conn: conn}
	reply := c.command(map[string]any{"connect": map[string]any{"token": opts.Token}})

	return c, reply.Error
}

/*
ConnectRealtime connects and fails the test unless the server accepts
the connection.
*/
func ConnectRealtime(tb testing.TB, baseURL string, opts RealtimeOptions) *Realtime {
	tb.Helper()

	c, refusal := DialRealtime(tb, baseURL, opts)
	require.Nil(tb, refusal, "the server refused the connection")

	return c
}

/*
Subscribe subscribes to the channel.
*/
func (c *Realtime) Subscribe(channel string) RealtimeSubscription {
	c.tb.Helper()

	return c.subscribeWith(map[string]any{"channel": channel})
}

/*
Recover subscribes to the channel asking for every publication after
the position.
*/
func (c *Realtime) Recover(channel, epoch string, offset uint64) RealtimeSubscription {
	c.tb.Helper()

	return c.subscribeWith(map[string]any{"channel": channel, "recover": true, "epoch": epoch, "offset": offset})
}

/*
Presence lists the connections listening on a channel the client is
subscribed to.
*/
func (c *Realtime) Presence(channel string) ([]RealtimeListener, *RealtimeError) {
	c.tb.Helper()

	reply := c.command(map[string]any{"presence": map[string]any{"channel": channel}})
	if reply.Error != nil {
		return nil, reply.Error
	}

	listeners := make([]RealtimeListener, 0, len(reply.Presence.Presence))
	for _, info := range reply.Presence.Presence {
		listeners = append(listeners, RealtimeListener{User: info.User, Info: info.ConnInfo})
	}

	return listeners, nil
}

/*
Publish tries to publish from the client and returns the server's
refusal.
*/
func (c *Realtime) Publish(channel string, data any) *RealtimeError {
	c.tb.Helper()

	return c.command(map[string]any{"publish": map[string]any{"channel": channel, "data": data}}).Error
}

/*
Publication returns the next publication, skipping other pushes.
*/
func (c *Realtime) Publication() RealtimePublication {
	c.tb.Helper()

	for {
		p := c.push()
		if p.Pub != nil {
			return c.decode(p.Channel, *p.Pub)
		}
	}
}

/*
Join returns the user of the next arrival on a channel, skipping other
pushes.
*/
func (c *Realtime) Join() string {
	c.tb.Helper()

	for {
		if p := c.push(); p.Join != nil {
			return p.Join.Info.User
		}
	}
}

/*
Unsubscribed returns the channel the server next took the connection
off, skipping other pushes.
*/
func (c *Realtime) Unsubscribed() string {
	c.tb.Helper()

	for {
		if p := c.push(); p.Unsub != nil {
			return p.Channel
		}
	}
}

/*
Closed reads until the server closes the connection and returns the
close code.
*/
func (c *Realtime) Closed() int {
	c.tb.Helper()

	ctx, cancel := context.WithTimeout(c.tb.Context(), RealtimeWait)
	defer cancel()

	for {
		if _, _, err := c.conn.Read(ctx); err != nil {
			return int(websocket.CloseStatus(err))
		}
	}
}

/*
ClosedBySubscribing asks for the channel without waiting for an answer,
reads until the server closes the connection instead, and returns the
close code.
*/
func (c *Realtime) ClosedBySubscribing(channel string) int {
	c.tb.Helper()

	c.id++

	data, err := json.Marshal(map[string]any{"id": c.id, "subscribe": map[string]any{"channel": channel}})
	require.NoError(c.tb, err)

	ctx, cancel := context.WithTimeout(c.tb.Context(), RealtimeWait)
	defer cancel()

	require.NoError(c.tb, c.conn.Write(ctx, websocket.MessageText, data))

	return c.Closed()
}

/*
Close closes the connection from the client's side.
*/
func (c *Realtime) Close() {
	c.tb.Helper()

	require.NoError(c.tb, c.conn.Close(websocket.StatusNormalClosure, ""))
}

func (c *Realtime) subscribeWith(params map[string]any) RealtimeSubscription {
	c.tb.Helper()

	reply := c.command(map[string]any{"subscribe": params})
	if reply.Error != nil {
		return RealtimeSubscription{Error: reply.Error}
	}

	sub := reply.Subscribe
	out := RealtimeSubscription{
		Recoverable: sub.Recoverable, Epoch: sub.Epoch, Offset: sub.Offset, Recovered: sub.Recovered,
	}

	channel, _ := params["channel"].(string)
	for _, pub := range sub.Publications {
		out.Publications = append(out.Publications, c.decode(channel, pub))
	}

	return out
}

/*
decode unwraps the envelope of a publication.
*/
func (c *Realtime) decode(channel string, pub realtimePub) RealtimePublication {
	c.tb.Helper()

	var envelope struct {
		ID    string            `json:"id"`
		Event string            `json:"event"`
		Data  json.RawMessage   `json:"data"`
		Meta  map[string]string `json:"meta"`
	}
	require.NoError(c.tb, json.Unmarshal(pub.Data, &envelope))

	return RealtimePublication{
		Channel: channel, Offset: pub.Offset, ID: envelope.ID, Event: envelope.Event, Data: envelope.Data,
		Meta: envelope.Meta,
	}
}
