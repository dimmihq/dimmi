package widget_test

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/tests"
)

func TestWidgetRealtime(t *testing.T) {
	t.Parallel()

	t.Run("a connect token opens the visitor's own channel only", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v, other := startVisitor(t, tc), startVisitor(t, tc)
		base := streamURL(t, tc)

		res := v.client().PostJSON("/widget/v1/realtime-tokens", nil).AssertCreated()
		res.AssertJSONPath("data.object", "realtime_token")
		res.AssertJSONPath("data.channel", "widget_contact:"+v.contact)

		s := dialStream(t, base, map[string]any{
			"token": res.String("data.token"),
			"subs":  map[string]any{"widget_contact:" + other.contact: map[string]any{"recover": true}},
		})
		connected := s.next()
		require.NotNil(t, connected.Connect)
		assert.Contains(t, connected.Connect.Subs, "widget_contact:"+v.contact)
		assert.NotContains(t, connected.Connect.Subs, "widget_contact:"+other.contact)
	})

	t.Run("a connect token is no bearer token for the API", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)
		token := v.client().PostJSON("/widget/v1/realtime-tokens", nil).AssertCreated().String("data.token")

		tc.Client().WithHeader("Authorization", "Bearer "+token).GetJSON("/widget/v1/session").
			AssertUnauthorized().AssertJSONPath("code", "session_invalid")
		tc.Client().WithHeader("Authorization", "Bearer "+token).GetJSON("/api/v1/me").AssertUnauthorized()
	})

	t.Run("an expired connect token or an ended session opens nothing", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)
		base := streamURL(t, tc)
		token := v.client().PostJSON("/widget/v1/realtime-tokens", nil).AssertCreated().String("data.token")

		tc.Clock.Travel(tc.App.Config.Widget.RealtimeTokenTTL * 2)
		refused := dialStream(t, base, map[string]any{"token": token}).next()
		require.NotNil(t, refused.Disconnect)

		fresh := v.client().PostJSON("/widget/v1/realtime-tokens", nil).AssertCreated().String("data.token")
		v.client().Delete("/widget/v1/session").AssertNoContent()
		require.NotNil(t, dialStream(t, base, map[string]any{"token": fresh}).next().Disconnect)
	})

	t.Run("signing out closes the open stream", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)
		s := v.openStream(t, streamURL(t, tc), nil)
		require.NotNil(t, s.next().Connect)

		v.client().Delete("/widget/v1/session").AssertNoContent()

		for {
			p := s.next()
			if p.Disconnect != nil {
				assert.Equal(t, 4009, p.Disconnect.Code)

				break
			}

			if p.Pub != nil {
				var notice struct {
					Event string          `json:"event"`
					Data  json.RawMessage `json:"data"`
				}
				require.NoError(t, json.Unmarshal(p.Pub.Data, &notice))
				assert.Equal(t, "widget.session_revoked", notice.Event)
				assert.JSONEq(t, `{"session_id":"`+v.id+`"}`, string(notice.Data))
			}
		}
	})

	t.Run("a stream past the session's limit closes the oldest one", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv("DIMMI_WIDGET__MAX_CONNECTIONS_PER_SESSION=2"))
		v, base := startVisitor(t, tc), streamURL(t, tc)
		oldest := v.openStream(t, base, nil)
		require.NotNil(t, oldest.next().Connect)
		kept := v.openStream(t, base, nil)
		require.NotNil(t, kept.next().Connect)

		require.NotNil(t, v.openStream(t, base, nil).next().Connect)

		for {
			p := oldest.next()
			if p.Disconnect != nil {
				assert.Equal(t, 4004, p.Disconnect.Code)

				break
			}
		}
	})

	t.Run("an ended session cannot get a token", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)
		v.client().Delete("/widget/v1/session").AssertNoContent()

		v.client().PostJSON("/widget/v1/realtime-tokens", testkit.JSON{}).AssertUnauthorized()
	})
}

func TestWidgetBroadcasts(t *testing.T) {
	t.Parallel()

	t.Run("a teammate's reply reaches the visitor live with its widget, a note never", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)
		id := v.startConversation(t, "hello")
		s := v.openStream(t, streamURL(t, tc), nil)
		require.NotNil(t, s.next().Connect)

		owner(tc).PostJSON("/api/v1/conversations/"+id+"/notes", testkit.JSON{"body": "secret note"}).AssertCreated()
		reply(t, tc, id, "Hi from Kim")

		event, data := s.eventOf("message.created")
		assert.Equal(t, "message.created", event)
		assert.NotContains(t, string(data), "secret note")
		assert.Contains(t, string(data), "Hi from Kim")
		assert.Contains(t, string(data), `"widget_id":"`+appID(t, tc)+`"`)
		assert.Contains(t, string(data), `"customer_status":"waiting_on_you"`)
		assert.NotContains(t, string(data), tc.Owner.Email)
	})

	t.Run("a reply too large to broadcast arrives as a reference to fetch", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)
		id := v.startConversation(t, "hello")
		s := v.openStream(t, streamURL(t, tc), nil)
		require.NotNil(t, s.next().Connect)

		reply(t, tc, id, strings.Repeat("long answer ", 1000))

		event, data := s.eventOf("message.created")
		assert.Equal(t, "message.created", event)
		assert.Contains(t, string(data), `"object":"message"`)
		assert.Contains(t, string(data), `"conversation_id":"`+id+`"`)
		assert.NotContains(t, string(data), "long answer")
	})

	t.Run("closing the conversation reaches the visitor", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)
		id := v.startConversation(t, "hello")
		s := v.openStream(t, streamURL(t, tc), nil)
		require.NotNil(t, s.next().Connect)

		owner(tc).PatchJSON("/api/v1/conversations/"+id, testkit.JSON{"status": "closed"}).AssertOK()

		for {
			event, data := s.event()
			if event == "conversation.closed" {
				assert.Contains(t, string(data), `"customer_status":"closed"`)

				break
			}
		}
	})

	t.Run("a reconnect recovers what it missed from its position", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)
		id := v.startConversation(t, "hello")
		base := streamURL(t, tc)
		channel := "widget_contact:" + v.contact

		first := v.openStream(t, base, nil).next()
		require.NotNil(t, first.Connect)
		position := first.Connect.Subs[channel]

		reply(t, tc, id, "while you were away")

		again := v.openStream(t, base, map[string]any{
			channel: map[string]any{"recover": true, "epoch": position.Epoch, "offset": position.Offset},
		}).next()
		require.NotNil(t, again.Connect)
		recovered := again.Connect.Subs[channel]
		assert.True(t, recovered.Recovered)
		require.NotEmpty(t, recovered.Publications)

		var missed []string
		for _, pub := range recovered.Publications {
			missed = append(missed, string(pub))
		}
		assert.Contains(t, strings.Join(missed, "\n"), "while you were away")
	})
}

func TestWidgetUpdates(t *testing.T) {
	t.Parallel()

	t.Run("polling catches up from the database without notes", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)
		id := v.startConversation(t, "hello")

		first := v.client().GetJSON("/widget/v1/updates").AssertOK()
		first.AssertJSONPath("data", []any{})
		cursor := first.String("meta.cursor")

		tc.Clock.Travel(time.Second)
		reply(t, tc, id, "polled reply")
		owner(tc).PostJSON("/api/v1/conversations/"+id+"/notes", testkit.JSON{"body": "secret note"}).AssertCreated()
		tc.Clock.Travel(time.Second)

		res := v.client().GetJSON("/widget/v1/updates?since=" + url.QueryEscape(cursor)).AssertOK()
		assert.Contains(t, string(res.Body), "polled reply")
		assert.NotContains(t, string(res.Body), "secret note")
		res.AssertJSONPath("data.0.event", "message.created")
		res.AssertJSONPath("meta.has_more", false)
		res.AssertJSONPath("meta.poll_after_ms", float64(tc.App.Config.Widget.PollingInterval.Milliseconds()))

		again := v.client().GetJSON("/widget/v1/updates?since=" + url.QueryEscape(res.String("meta.cursor"))).AssertOK()
		assert.NotContains(t, string(again.Body), "polled reply")
	})

	t.Run("a forged cursor is refused", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)

		v.client().GetJSON("/widget/v1/updates?since=forged").AssertStatus(fhttp.StatusBadRequest)
	})
}
