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
command sends a command and returns its reply, queueing the pushes that
arrive first.
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
		f := c.next()
		if f.ID == c.id {
			return f
		}

		c.queue = append(c.queue, f)
	}
}

/*
push returns the next push, queued or read.
*/
func (c *Realtime) push() realtimePush {
	c.tb.Helper()

	for {
		var f realtimeFrame
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
next reads one frame, queueing the others of the same message. A ping
is an empty frame, which callers skip.
*/
func (c *Realtime) next() realtimeFrame {
	c.tb.Helper()

	ctx, cancel := context.WithTimeout(c.tb.Context(), RealtimeWait)
	defer cancel()

	_, data, err := c.conn.Read(ctx)
	require.NoError(c.tb, err)

	lines := bytes.Split(bytes.TrimSpace(data), []byte("\n"))
	frames := make([]realtimeFrame, len(lines))

	for i, line := range lines {
		require.NoError(c.tb, json.Unmarshal(line, &frames[i]))
	}

	c.queue = append(c.queue, frames[1:]...)

	return frames[0]
}
