package widget_test

import (
	"encoding/json"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/broadcasting"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/tests"
)

/*
streamPush is one push a visitor's stream receives: the connect answer,
a publication on a channel, or the reason it ends.
*/
type streamPush struct {
	Channel string `json:"channel"`
	Pub     *struct {
		Offset uint64          `json:"offset"`
		Data   json.RawMessage `json:"data"`
	} `json:"pub"`
	Connect *struct {
		Subs map[string]struct {
			Recovered    bool              `json:"recovered"`
			Epoch        string            `json:"epoch"`
			Offset       uint64            `json:"offset"`
			Publications []json.RawMessage `json:"publications"`
		} `json:"subs"`
	} `json:"connect"`
	Disconnect *struct {
		Code   int    `json:"code"`
		Reason string `json:"reason"`
	} `json:"disconnect"`
}

/*
stream is a visitor's open event stream.
*/
type stream struct {
	t      *testing.T
	events *httpclient.SSEReader
}

/*
openStream opens the visitor's stream the way their messenger does: a fresh
connect token, then the event source with the connect command in its URL.
*/
func (v *visitor) openStream(t *testing.T, base string, subs map[string]any) *stream {
	t.Helper()

	token := v.client().PostJSON("/widget/v1/realtime-tokens", nil).AssertCreated().String("data.token")

	return dialStream(t, base, map[string]any{"token": token, "subs": subs})
}

/*
dialStream opens the event stream with the connect command.
*/
func dialStream(t *testing.T, base string, connect map[string]any) *stream {
	t.Helper()

	data, err := json.Marshal(connect)
	require.NoError(t, err)

	client, err := httpclient.New(httpclient.Config{AllowPrivateNetworks: true})
	require.NoError(t, err)

	resp, err := client.New().Stream().Timeout(testkit.RealtimeWait).
		Get(t.Context(), base+broadcasting.PathUniSSE+"?cf_connect="+url.QueryEscape(string(data)))
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Close() })

	return &stream{t: t, events: httpclient.SSE(resp)}
}

/*
next returns the next push that is not a ping.
*/
func (s *stream) next() streamPush {
	s.t.Helper()

	for {
		event, err := s.events.Next()
		require.NoError(s.t, err)

		var p streamPush
		require.NoError(s.t, json.Unmarshal([]byte(event.Data), &p))

		if p.Pub != nil || p.Connect != nil || p.Disconnect != nil {
			return p
		}
	}
}

/*
event returns the next publication's event name and data.
*/
func (s *stream) event() (string, json.RawMessage) {
	s.t.Helper()

	for {
		p := s.next()
		require.Nil(s.t, p.Disconnect, "stream ended")

		if p.Pub == nil {
			continue
		}

		var envelope struct {
			Event string          `json:"event"`
			Data  json.RawMessage `json:"data"`
		}
		require.NoError(s.t, json.Unmarshal(p.Pub.Data, &envelope))

		return envelope.Event, envelope.Data
	}
}

/*
streamURL serves the application's event stream endpoint on a local port.
*/
func streamURL(t *testing.T, tc *tests.TestCase) string {
	t.Helper()

	return testkit.Serve(t, tc.App.Broadcast.Handlers()[broadcasting.PathUniSSE])
}
