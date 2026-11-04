package broadcasting_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/broadcasting"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/log"
	"github.com/dimmihq/dimmi/framework/redis"
)

/*
waitFor bounds every read from a test client.
*/
const waitFor = 10 * time.Second

/*
node is a running server reachable over HTTP.
*/
type node struct {
	server *broadcasting.Server
	url    string
	logs   *syncBuffer
}

/*
syncBuffer collects log lines written from several goroutines.
*/
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.String()
}

/*
resolve authenticates test connections from their token: "user" or
"user|channel,channel" connects as that user and subscribes the listed
channels, "anonymous" connects with no user, "fail" breaks the resolver,
"badinfo" returns info that cannot be encoded, and no token is
unauthenticated.
*/
func resolve(_ context.Context, _ *http.Request, token string) (broadcasting.Principal, error) {
	switch token {
	case "":
		return broadcasting.Principal{}, broadcasting.ErrUnauthenticated
	case "anonymous":
		return broadcasting.Principal{}, nil
	case "fail":
		return broadcasting.Principal{}, errors.New("resolver is down")
	case "badinfo":
		return broadcasting.Principal{UserID: "x", Info: make(chan int)}, nil
	}

	user, channels, _ := strings.Cut(token, "|")
	principal := broadcasting.Principal{UserID: user, Info: map[string]string{"name": user}, Subject: user}

	if channels != "" {
		principal.Channels = strings.Split(channels, ",")
	}

	return principal, nil
}

/*
newNode starts a server on the client's Valkey with the test channels:
"room:{id}" tracks presence and keeps history, and refuses the room
"secret" and fails for the room "broken"; "user:{id}" keeps history and
admits only that user.
*/
func newNode(t *testing.T, client *redis.Client) *node {
	t.Helper()

	logs := &syncBuffer{}
	logger, err := log.New(log.Options{Driver: "json", Level: "debug", Writer: logs})
	require.NoError(t, err)

	server, err := broadcasting.Open(log.NewContext(t.Context(), logger), client, broadcasting.Config{Resolve: resolve})
	require.NoError(t, err)
	t.Cleanup(func() { _ = server.Shutdown(context.WithoutCancel(t.Context())) })

	room := broadcasting.ChannelOptions{Presence: true, JoinLeave: true, HistorySize: 10, HistoryTTL: time.Minute}
	require.NoError(t, server.Channels().Authorize("room:{id}", room, authorizeRoom))

	user := broadcasting.ChannelOptions{HistorySize: 10, HistoryTTL: time.Minute}
	require.NoError(t, server.Channels().Authorize("user:{id}", user, authorizeUser))

	mux := http.NewServeMux()
	for path, handler := range server.Handlers() {
		mux.Handle(path, handler)
	}

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return &node{server: server, url: srv.URL, logs: logs}
}

func authorizeRoom(_ context.Context, _ broadcasting.Principal, params map[string]string) (bool, error) {
	if params["id"] == "broken" {
		return false, errors.New("authorizer is down")
	}

	return params["id"] != "secret", nil
}

func authorizeUser(_ context.Context, principal broadcasting.Principal, params map[string]string) (bool, error) {
	return principal.Subject == params["id"], nil
}

/*
protoError is an error reply of the client protocol.
*/
type protoError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

/*
publication is one broadcast as clients receive it.
*/
type publication struct {
	Data   json.RawMessage `json:"data"`
	Offset uint64          `json:"offset"`
}

/*
subscription is the result of subscribing to a channel.
*/
type subscription struct {
	Recoverable  bool          `json:"recoverable"`
	Epoch        string        `json:"epoch"`
	Offset       uint64        `json:"offset"`
	Publications []publication `json:"publications"`
	Recovered    bool          `json:"recovered"`
}

/*
connected is the result of connecting.
*/
type connected struct {
	Client string                  `json:"client"`
	Subs   map[string]subscription `json:"subs"`
}

/*
clientInfo is a connection in a join or leave notice.
*/
type clientInfo struct {
	User     string          `json:"user"`
	ConnInfo json.RawMessage `json:"conn_info"`
}

/*
push is something the server sends on its own.
*/
type push struct {
	Channel string       `json:"channel"`
	Pub     *publication `json:"pub"`
	Join    *struct {
		Info clientInfo `json:"info"`
	} `json:"join"`
	Connect    *connected  `json:"connect"`
	Disconnect *protoError `json:"disconnect"`
}

/*
frame is one reply or push of the bidirectional protocol.
*/
type frame struct {
	ID        int           `json:"id"`
	Error     *protoError   `json:"error"`
	Connect   *connected    `json:"connect"`
	Subscribe *subscription `json:"subscribe"`
	Presence  *struct {
		Presence map[string]clientInfo `json:"presence"`
	} `json:"presence"`
	Push *push `json:"push"`
}

/*
wsClient speaks the JSON client protocol over WebSocket.
*/
type wsClient struct {
	t     *testing.T
	conn  *websocket.Conn
	id    int
	queue []frame
}

/*
dial connects to the node over WebSocket and sends the connect command
with the token.
*/
func dial(t *testing.T, n *node, token string) (*wsClient, frame) {
	t.Helper()

	conn, err := upgrade(t, n, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.CloseNow() })

	c := &wsClient{t: t, conn: conn}
	reply := c.command(map[string]any{"connect": map[string]any{"token": token}})

	return c, reply
}

/*
upgrade opens a WebSocket to the node with the extra request headers.
*/
func upgrade(t *testing.T, n *node, header http.Header) (*websocket.Conn, error) {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), waitFor)
	defer cancel()

	target := "ws" + strings.TrimPrefix(n.url, "http") + broadcasting.PathWebsocket

	conn, resp, err := websocket.Dial(ctx, target, &websocket.DialOptions{HTTPHeader: header})
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}

	return conn, err
}

/*
connect dials and fails the test unless the connection is accepted.
*/
func connect(t *testing.T, n *node, token string) *wsClient {
	t.Helper()

	c, reply := dial(t, n, token)
	require.Nil(t, reply.Error)
	require.NotNil(t, reply.Connect)

	return c
}

/*
command sends a command and returns its reply, queueing pushes that
arrive first.
*/
func (c *wsClient) command(cmd map[string]any) frame {
	c.t.Helper()

	c.id++
	cmd["id"] = c.id

	data, err := json.Marshal(cmd)
	require.NoError(c.t, err)

	ctx, cancel := context.WithTimeout(c.t.Context(), waitFor)
	defer cancel()

	require.NoError(c.t, c.conn.Write(ctx, websocket.MessageText, data))

	for {
		f := c.next()
		if f.ID == c.id {
			return f
		}

		c.queue = append(c.queue, f)
	}
}

/*
subscribe subscribes to the channel, recovering from the position when
an epoch is given.
*/
func (c *wsClient) subscribe(channel, epoch string, offset uint64) frame {
	c.t.Helper()

	params := map[string]any{"channel": channel}
	if epoch != "" {
		params["recover"] = true
		params["epoch"] = epoch
		params["offset"] = offset
	}

	return c.command(map[string]any{"subscribe": params})
}

/*
push returns the next push, answering pings on the way.
*/
func (c *wsClient) push() push {
	c.t.Helper()

	for {
		var f frame
		if len(c.queue) > 0 {
			f, c.queue = c.queue[0], c.queue[1:]
		} else {
			f = c.next()
		}

		if f.Push != nil {
			return *f.Push
		}
	}
}

/*
publication returns the next publication, skipping other pushes.
*/
func (c *wsClient) publication() push {
	c.t.Helper()

	for {
		if p := c.push(); p.Pub != nil {
			return p
		}
	}
}

/*
join returns the user of the next join notice, skipping other pushes.
*/
func (c *wsClient) join() string {
	c.t.Helper()

	for {
		if p := c.push(); p.Join != nil {
			return p.Join.Info.User
		}
	}
}

/*
next reads one frame, splitting a message that carries several.
*/
func (c *wsClient) next() frame {
	c.t.Helper()

	ctx, cancel := context.WithTimeout(c.t.Context(), waitFor)
	defer cancel()

	_, data, err := c.conn.Read(ctx)
	require.NoError(c.t, err)

	lines := bytes.Split(bytes.TrimSpace(data), []byte("\n"))
	frames := make([]frame, len(lines))

	for i, line := range lines {
		require.NoError(c.t, json.Unmarshal(line, &frames[i]))
	}

	c.queue = append(c.queue, frames[1:]...)

	return frames[0]
}

/*
closed reads until the server closes the connection and returns the
close code.
*/
func (c *wsClient) closed() websocket.StatusCode {
	c.t.Helper()

	ctx, cancel := context.WithTimeout(c.t.Context(), waitFor)
	defer cancel()

	for {
		if _, _, err := c.conn.Read(ctx); err != nil {
			return websocket.CloseStatus(err)
		}
	}
}

/*
envelope is the broadcast as the server encodes it.
*/
type envelope struct {
	ID         string            `json:"id"`
	Event      string            `json:"event"`
	ServerTime time.Time         `json:"server_time"`
	Data       json.RawMessage   `json:"data"`
	Meta       map[string]string `json:"meta"`
}

/*
decode reads the envelope of a publication.
*/
func decode(t *testing.T, pub *publication) envelope {
	t.Helper()
	require.NotNil(t, pub)

	var e envelope
	require.NoError(t, json.Unmarshal(pub.Data, &e))

	return e
}

/*
sseClient is a plain EventSource-like listener.
*/
type sseClient struct {
	t      *testing.T
	events *httpclient.SSEReader
}

/*
listen opens the server-sent events endpoint with the connect command.
*/
func listen(t *testing.T, n *node, connect map[string]any) *sseClient {
	t.Helper()

	resp := openSSE(t, n, connect)
	require.Equal(t, http.StatusOK, resp.Status)
	require.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))

	return &sseClient{t: t, events: httpclient.SSE(resp)}
}

func openSSE(t *testing.T, n *node, connect map[string]any) *httpclient.Response {
	t.Helper()

	client, err := httpclient.New(httpclient.Config{AllowPrivateNetworks: true})
	require.NoError(t, err)

	target := n.url + broadcasting.PathUniSSE
	if connect != nil {
		data, err := json.Marshal(connect)
		require.NoError(t, err)

		target += "?cf_connect=" + url.QueryEscape(string(data))
	}

	resp, err := client.New().Stream().Timeout(waitFor).Get(t.Context(), target)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Close() })

	return resp
}

/*
push returns the next push that is not a ping.
*/
func (c *sseClient) push() push {
	c.t.Helper()

	for {
		event, err := c.events.Next()
		require.NoError(c.t, err)

		var p push
		require.NoError(c.t, json.Unmarshal([]byte(event.Data), &p))

		if p != (push{}) {
			return p
		}
	}
}
