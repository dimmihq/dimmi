/*
|--------------------------------------------------------------------------
| WebSocket Client
|--------------------------------------------------------------------------
|
| Some services push to the application over a WebSocket the
| application opens: chat gateways, streaming APIs. The address often
| comes from configuration or from a user, so the connection leaves
| through the same guarded HTTP client as every other outbound request
| and can never be pointed at the database, the cache or the cloud
| metadata service.
|
| Errors name the host only, since socket URLs often carry tokens.
|
*/

package websocket

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/coder/websocket"

	"github.com/dimmihq/dimmi/framework/httpclient"
)

/*
MessageType says how a message's bytes are meant: UTF-8 text or binary.
*/
type MessageType int

/*
The message types.
*/
const (
	Text MessageType = iota + 1
	Binary
)

/*
CloseError is returned by Read once the peer closed the connection, with
the close code and reason it sent. Gateways use the code to decide
whether to resume, reconnect or give up.
*/
type CloseError struct {
	Code   int
	Reason string
}

/*
Error describes the close.
*/
func (e *CloseError) Error() string {
	return fmt.Sprintf("websocket: closed with %d %s", e.Code, e.Reason)
}

/*
Options configure a dial.

HTTP is the client whose connection checks the dial goes through; left
nil, only public addresses are reached. Header and Subprotocols go into
the handshake. ReadLimit caps one incoming message in bytes when
positive, and is 32 KiB otherwise.
*/
type Options struct {
	HTTP         *httpclient.Client
	Header       http.Header
	Subprotocols []string
	ReadLimit    int64
}

/*
Conn is an open WebSocket connection. One goroutine may read while
another writes.
*/
type Conn struct {
	conn *websocket.Conn
}

/*
Dial opens a WebSocket connection to the ws or wss URL.
*/
func Dial(ctx context.Context, rawURL string, opts Options) (*Conn, error) {
	target, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("dial websocket: %w", err)
	}

	client := opts.HTTP
	if client == nil {
		if client, err = httpclient.New(httpclient.Config{}); err != nil {
			return nil, fmt.Errorf("dial websocket %s: %w", target.Host, err)
		}
	}

	conn, resp, err := websocket.Dial(ctx, rawURL, &websocket.DialOptions{
		HTTPClient:   client.StdClient(),
		HTTPHeader:   opts.Header,
		Subprotocols: opts.Subprotocols,
	})
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}

	if err != nil {
		return nil, fmt.Errorf("dial websocket %s: %w", target.Host, err)
	}

	if opts.ReadLimit > 0 {
		conn.SetReadLimit(opts.ReadLimit)
	}

	return &Conn{conn: conn}, nil
}

/*
Subprotocol returns the subprotocol the server chose, or an empty
string.
*/
func (c *Conn) Subprotocol() string {
	return c.conn.Subprotocol()
}

/*
Read waits for the next message. It returns a CloseError once the peer
closed the connection.
*/
func (c *Conn) Read(ctx context.Context) (MessageType, []byte, error) {
	typ, data, err := c.conn.Read(ctx)
	if err != nil {
		return 0, nil, readError(err)
	}

	if typ == websocket.MessageBinary {
		return Binary, data, nil
	}

	return Text, data, nil
}

/*
Write sends one message.
*/
func (c *Conn) Write(ctx context.Context, typ MessageType, data []byte) error {
	kind := websocket.MessageText
	if typ == Binary {
		kind = websocket.MessageBinary
	}

	if err := c.conn.Write(ctx, kind, data); err != nil {
		return fmt.Errorf("write websocket message: %w", err)
	}

	return nil
}

/*
Close closes the connection normally, waiting briefly for the peer to
acknowledge.
*/
func (c *Conn) Close() error {
	if err := c.conn.Close(websocket.StatusNormalClosure, ""); err != nil {
		return fmt.Errorf("close websocket: %w", err)
	}

	return nil
}

func readError(err error) error {
	if closed, ok := errors.AsType[websocket.CloseError](err); ok {
		return &CloseError{Code: int(closed.Code), Reason: closed.Reason}
	}

	return fmt.Errorf("read websocket message: %w", err)
}
