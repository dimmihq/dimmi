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
read reads one message and queues its frames in order. The server may
batch several frames in one message, so a reply can arrive behind a push.
A ping is an empty frame, which callers skip.
*/
func (c *Realtime) read() {
	c.tb.Helper()

	ctx, cancel := context.WithTimeout(c.tb.Context(), RealtimeWait)
	defer cancel()

	_, data, err := c.conn.Read(ctx)
	require.NoError(c.tb, err)

	for line := range bytes.SplitSeq(bytes.TrimSpace(data), []byte("\n")) {
		var f realtimeFrame
		require.NoError(c.tb, json.Unmarshal(line, &f))
		c.queue = append(c.queue, f)
	}
}
