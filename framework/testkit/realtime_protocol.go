/*
|--------------------------------------------------------------------------
| Realtime Test Protocol
|--------------------------------------------------------------------------
|
| The frames of the JSON client protocol a realtime test client reads:
| command replies, matched to their command by id, and pushes the
| server sends on its own, such as publications, arrivals and the
| disconnect. Several frames may share one WebSocket message, one per
| line.
|
*/

package testkit

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"

	"github.com/coder/websocket"
	"github.com/stretchr/testify/require"
)

/*
RealtimeError is an error the server answered a command with, or the
reason it gave for a disconnect.
*/
type RealtimeError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

/*
RealtimeListener is one connection listening on a channel: its user
and the info its server gave about it.
*/
type RealtimeListener struct {
	User string
	Info json.RawMessage
}

type realtimePub struct {
	Data   json.RawMessage `json:"data"`
	Offset uint64          `json:"offset"`
}

type realtimeClientInfo struct {
	User     string          `json:"user"`
	ConnInfo json.RawMessage `json:"conn_info"`
}

type realtimePush struct {
	Channel string       `json:"channel"`
	Pub     *realtimePub `json:"pub"`
	Join    *struct {
		Info realtimeClientInfo `json:"info"`
	} `json:"join"`
	Unsub *struct {
		Code int `json:"code"`
	} `json:"unsubscribe"`
}

type realtimeFrame struct {
	ID        int            `json:"id"`
	Error     *RealtimeError `json:"error"`
	Subscribe *struct {
		Recoverable  bool          `json:"recoverable"`
		Epoch        string        `json:"epoch"`
		Offset       uint64        `json:"offset"`
		Recovered    bool          `json:"recovered"`
		Publications []realtimePub `json:"publications"`
	} `json:"subscribe"`
	Presence *struct {
		Presence map[string]realtimeClientInfo `json:"presence"`
	} `json:"presence"`
	Push *realtimePush `json:"push"`
}

/*
command sends a command and returns its reply, wherever it sits among the
frames read so far, leaving the pushes queued in the order they came.
*/
func (c *Realtime) command(cmd map[string]any) realtimeFrame {
	c.tb.Helper()

	c.id++
	cmd["id"] = c.id

	data, err := json.Marshal(cmd)
	require.NoError(c.tb, err)

	ctx, cancel := context.WithTimeout(c.tb.Context(), RealtimeWait)
	defer cancel()

	require.NoError(c.tb, c.conn.Write(ctx, websocket.MessageText, data))

	for {
		for i, f := range c.queue {
			if f.ID == c.id {
				c.queue = append(c.queue[:i], c.queue[i+1:]...)

				return f
			}
		}

		c.read()
	}
}

/*
push returns the next push, queued or read.
*/
func (c *Realtime) push() realtimePush {
	c.tb.Helper()

	for {
		if len(c.queue) == 0 {
			c.read()

			continue
		}

		f := c.queue[0]
		c.queue = c.queue[1:]

		if f.Push != nil {
			return *f.Push
		}
	}
}

/*
realtimeBacklog is how many messages the reader holds before a test takes
them.
*/
const realtimeBacklog = 1024

/*
realtimeMessage is one message the reader took off the connection: its
frames, or the error that ended the connection.
*/
type realtimeMessage struct {
	frames []realtimeFrame
	err    error
}

/*
listen reads the connection for as long as it stays open, so the
server's pings are answered while a test is busy elsewhere. A ping is an
empty frame and the pong is one too; the server drops a client that
misses one. Every other frame is handed to the test, in order.
*/
func (c *Realtime) listen() {
	for {
		_, data, err := c.conn.Read(context.Background())
		if err != nil {
			c.received <- realtimeMessage{err: err}

			return
		}

		if m := c.frames(data); len(m.frames) > 0 || m.err != nil {
			c.received <- m
		}
	}
}

/*
frames splits one message into its frames, answering a ping on the way.
*/
func (c *Realtime) frames(data []byte) realtimeMessage {
	var m realtimeMessage

	for line := range bytes.SplitSeq(bytes.TrimSpace(data), []byte("\n")) {
		if bytes.Equal(line, []byte("{}")) {
			m.err = cmp.Or(m.err, c.pong())

			continue
		}

		var f realtimeFrame

		m.err = cmp.Or(m.err, json.Unmarshal(line, &f))
		m.frames = append(m.frames, f)
	}

	return m
}

/*
pong answers a ping.
*/
func (c *Realtime) pong() error {
	ctx, cancel := context.WithTimeout(context.Background(), RealtimeWait)
	defer cancel()

	return c.conn.Write(ctx, websocket.MessageText, []byte("{}"))
}

/*
read waits for the reader's next message and queues its frames in order.
The server may batch several frames in one message, so a reply can arrive
behind a push.
*/
func (c *Realtime) read() {
	c.tb.Helper()

	ctx, cancel := context.WithTimeout(c.tb.Context(), RealtimeWait)
	defer cancel()

	select {
	case m := <-c.received:
		require.NoError(c.tb, m.err)

		c.queue = append(c.queue, m.frames...)
	case <-ctx.Done():
		require.FailNow(c.tb, "no realtime message arrived")
	}
}
