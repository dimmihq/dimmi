package channels_test

import (
	"encoding/json"
	"io"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
	"github.com/dimmihq/dimmi/framework/testkit"

	wadriver "github.com/dimmihq/dimmi/app/drivers/channel/whatsapp"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

/*
The Meta app and number the WhatsApp tests connect.
*/
const (
	waAppID     = "999001"
	waSecret    = "wa-app-secret" //nolint:gosec // a test value the fake signs with
	waVerify    = "wa-verify-token"
	waToken     = "EAAG-test-token" //nolint:gosec // a test token the fake accepts
	waAccount   = "WABA1"
	waNumber    = "PN1"
	waCustomer  = "447700900123"
	waUserID    = "GB.42"
	graphPrefix = "https://graph.facebook.com/v25.0/"
)

/*
graphCall is one call the fake Graph API received.
*/
type graphCall struct {
	Method string
	Path   string
	Body   map[string]any
}

/*
fakeGraph answers the Graph API calls the channel makes and records
them.
*/
type fakeGraph struct {
	mu        sync.Mutex
	calls     []graphCall
	sent      int
	refuseTpl bool
	written   map[string]string
}

func (f *fakeGraph) serve(w fhttp.ResponseWriter, r *fhttp.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v25.0/")
	call := graphCall{Method: r.Method, Path: path}

	if r.Body != nil {
		if raw, _ := io.ReadAll(r.Body); strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			_ = json.Unmarshal(raw, &call.Body)
		}
	}

	f.mu.Lock()
	f.calls = append(f.calls, call)
	f.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")

	if f.serveTemplates(w, call) {
		return
	}

	switch {
	case path == "debug_token":
		_, _ = io.WriteString(w, `{"data":{"app_id":"`+waAppID+`","is_valid":true,`+
			`"scopes":["whatsapp_business_management","whatsapp_business_messaging"]}}`)
	case path == waAccount:
		_, _ = io.WriteString(w, `{"id":"`+waAccount+`","name":"Acme WhatsApp",`+
			`"owner_business_info":{"id":"B1","name":"Acme"}}`)
	case path == waAccount+"/phone_numbers":
		_, _ = io.WriteString(w, `{"data":[{"id":"`+waNumber+`","display_phone_number":"+1 555-000-1111",`+
			`"verified_name":"Acme Support","name_status":"APPROVED","quality_rating":"GREEN"}]}`)
	case path == waAccount+"/subscribed_apps":
		_, _ = io.WriteString(w, `{"success":true}`)
	case path == waAccount+"/message_templates":
		status := "APPROVED"
		if f.refusing() {
			status = "PAUSED"
		}

		_, _ = io.WriteString(w, `{"data":[{"id":"T1","name":"order_update","language":"en_US","status":"`+status+`",`+
			`"category":"UTILITY","components":[{"type":"BODY","text":"Hi {{1}}, order {{2}} shipped."}]},`+
			`{"id":"T2","name":"promo","language":"en_US","status":"APPROVED","category":"MARKETING",`+
			`"components":[{"type":"HEADER","format":"IMAGE"},{"type":"BODY","text":"Sale"}]}`+f.writtenJSON()+`]}`)
	case path == waNumber+"/messages" && call.Body["status"] != nil:
		_, _ = io.WriteString(w, `{"success":true}`)
	case path == waNumber+"/messages" && call.Body["type"] == "template" && f.refusing():
		w.WriteHeader(fhttp.StatusBadRequest)
		_, _ = io.WriteString(w, `{"error":{"code":132000,"message":"Number of parameters does not match"}}`)
	case path == waNumber+"/messages":
		f.mu.Lock()
		f.sent++
		id := "wamid.OUT" + strconv.Itoa(f.sent)
		f.mu.Unlock()
		_, _ = io.WriteString(w, `{"messages":[{"id":"`+id+`"}]}`)
	default:
		w.WriteHeader(fhttp.StatusNotFound)
		_, _ = io.WriteString(w, `{"error":{"code":100,"message":"unknown path"}}`)
	}
}

/*
sends returns the messages the channel sent to customers.
*/
func (f *fakeGraph) sends() []graphCall {
	f.mu.Lock()
	defer f.mu.Unlock()

	var out []graphCall
	for _, c := range f.calls {
		if c.Path == waNumber+"/messages" && c.Body["status"] == nil {
			out = append(out, c)
		}
	}

	return out
}

/*
refusing reports whether Meta refuses templates, as when a template
changed in WhatsApp Manager after dimmi copied it.
*/
func (f *fakeGraph) refusing() bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.refuseTpl
}

/*
readMark returns the message the channel last marked read.
*/
func (f *fakeGraph) readMark() string {
	f.mu.Lock()
	defer f.mu.Unlock()

	out := ""
	for _, c := range f.calls {
		if c.Body["status"] == "read" {
			out, _ = c.Body["message_id"].(string)
		}
	}

	return out
}

/*
waCase is a test with Meta faked and a WhatsApp number connected.
*/
type waCase struct {
	*slackCase

	graph *fakeGraph
}

func newWhatsAppCase(t *testing.T) *waCase {
	t.Helper()

	tc := tests.New(t, tests.WithEnv(
		"DIMMI_SERVICES__WHATSAPP__APP_ID="+waAppID, "DIMMI_SERVICES__WHATSAPP__APP_SECRET="+waSecret,
		"DIMMI_SERVICES__WHATSAPP__VERIFY_TOKEN="+waVerify,
	))
	graph := &fakeGraph{}
	httpclienttest.New(tc.App.HTTP).Serve("https://graph.facebook.com/*", fhttp.HandlerFunc(graph.serve))

	wc := &waCase{slackCase: &slackCase{TestCase: tc}, graph: graph}
	res := tc.ActingAsOwner().PostJSON("/api/v1/channels/whatsapp", map[string]any{
		"access_token": waToken, "waba_id": waAccount, "phone_number_id": waNumber,
	}).AssertCreated()

	id, err := tc.App.IDs.Decode(string(models.PrefixChannelConnection), res.String("data.id"))
	require.NoError(t, err)
	wc.conn = wc.connection(id)

	return wc
}

/*
deliverWA posts a signed messages change to the shared address and
processes it.
*/
func (wc *waCase) deliverWA(value map[string]any) *testkit.Response {
	wc.T.Helper()

	value["messaging_product"] = "whatsapp"
	value["metadata"] = map[string]any{"display_phone_number": "15550001111", "phone_number_id": waNumber}
	raw, _ := json.Marshal(map[string]any{
		"object": "whatsapp_business_account",
		"entry": []any{map[string]any{
			"id": waAccount, "changes": []any{map[string]any{"field": "messages", "value": value}},
		}},
	})

	res := wc.Client().WithHeader("Content-Type", "application/json").
		WithHeader(wadriver.SignatureHeader, wadriver.Sign(waSecret, raw)).Send("POST", "/webhooks/whatsapp", raw)
	wc.Drain()

	return res
}

/*
customerText is the customer's text message wamid.IN1 at the time.
*/
func customerText(text string, at time.Time) map[string]any {
	return map[string]any{
		"contacts": []any{map[string]any{
			"profile": map[string]any{"name": "Ada Lovelace"}, "wa_id": waCustomer, "user_id": waUserID,
		}},
		"messages": []any{map[string]any{
			"from": waCustomer, "from_user_id": waUserID, "id": "wamid.IN1", "type": "text",
			"timestamp": formatUnix(at), "text": map[string]any{"body": text},
		}},
	}
}

func formatUnix(at time.Time) string {
	return strconv.FormatInt(at.Unix(), 10)
}

/*
field returns an object inside a sent body.
*/
func field(t *testing.T, body map[string]any, key string) map[string]any {
	t.Helper()

	out, ok := body[key].(map[string]any)
	require.True(t, ok, "%s is an object", key)

	return out
}

/*
replies keeps the messages, leaving out timeline events.
*/
func replies(all []models.Message) []models.Message {
	var out []models.Message
	for _, m := range all {
		if m.Type == "message" {
			out = append(out, m)
		}
	}

	return out
}

func TestWhatsAppChannel(t *testing.T) {
	t.Parallel()

	t.Run("connects a number with a token and copies its templates", func(t *testing.T) {
		t.Parallel()
		wc := newWhatsAppCase(t)

		facts := map[string]any{}
		require.NoError(t, json.Unmarshal(wc.conn.Config, &facts))
		assert.Equal(t, "active", wc.conn.Status)
		assert.Equal(t, waNumber, facts["phone_number_id"])
		assert.Equal(t, "shared", facts["mode"])

		res := wc.ActingAsOwner().GetJSON("/api/v1/channels/whatsapp/" + wc.publicID() + "/templates").AssertOK()
		assert.Equal(t, "order_update", res.String("data.0.name"))
		assert.Equal(t, "true", res.String("data.0.sendable"))
		assert.Equal(t, "false", res.String("data.1.sendable"))

		wc.ActingAsOwner().PostJSON("/api/v1/channels/whatsapp", map[string]any{
			"access_token": waToken, "waba_id": waAccount, "phone_number_id": waNumber,
		}).AssertStatus(409)
	})

	t.Run("answers Meta's address check only with the right token", func(t *testing.T) {
		t.Parallel()
		wc := newWhatsAppCase(t)

		ok := wc.Client().Send("GET", "/webhooks/whatsapp?hub.mode=subscribe&hub.verify_token="+waVerify+
			"&hub.challenge=c123", nil).AssertOK()
		assert.Equal(t, "c123", string(ok.Body))
		wc.Client().Send("GET", "/webhooks/whatsapp?hub.mode=subscribe&hub.verify_token=wrong&hub.challenge=c1", nil).
			AssertStatus(403)
	})

	t.Run("refuses an unsigned delivery", func(t *testing.T) {
		t.Parallel()
		wc := newWhatsAppCase(t)
		raw := []byte(`{"object":"whatsapp_business_account","entry":[]}`)

		wc.Client().Send("POST", "/webhooks/whatsapp", raw).AssertUnauthorized()
		wc.Client().WithHeader(wadriver.SignatureHeader, wadriver.Sign("other", raw)).
			Send("POST", "/webhooks/whatsapp", raw).AssertUnauthorized()
	})

	t.Run("a customer's message opens a conversation once and opens their window", func(t *testing.T) {
		t.Parallel()
		wc := newWhatsAppCase(t)
		msg := customerText("My *order* has not arrived", wc.Clock.Now())

		wc.deliverWA(msg).AssertOK()
		wc.deliverWA(customerText("My *order* has not arrived", wc.Clock.Now()))

		convs := wc.conversations()
		require.Len(t, convs, 1)
		assert.Equal(t, "whatsapp:u:"+waUserID, *convs[0].ExternalThreadKey)

		msgs := wc.messages(convs[0])
		require.Len(t, msgs, 1)
		assert.Equal(t, "My **order** has not arrived", body(msgs[0]))

		convID := models.PublicID(models.PrefixConversation, convs[0].ID)
		res := wc.ActingAsOwner().GetJSON("/api/v1/conversations/" + convID + "/whatsapp").AssertOK()
		assert.Equal(t, "true", res.String("data.window.open"))
		assert.Equal(t, "Ada Lovelace", res.String("data.contact.name"))
	})

	t.Run("replies within the window, and follows Meta's status reports", func(t *testing.T) {
		t.Parallel()
		wc := newWhatsAppCase(t)
		wc.deliverWA(customerText("hello", wc.Clock.Now()))
		conv := wc.conversations()[0]
		convID := models.PublicID(models.PrefixConversation, conv.ID)

		wc.ActingAsOwner().PostJSON("/api/v1/conversations/"+convID+"/replies", map[string]any{
			"body": "**Sorry** about that, checking now.",
		}).AssertCreated()
		wc.Drain()

		sends := wc.graph.sends()
		require.Len(t, sends, 1)
		assert.Equal(t, waCustomer, sends[0].Body["to"])
		assert.Equal(t, "wamid.IN1", wc.graph.readMark(), "the customer's message is marked read before the reply")
		assert.Equal(t, "*Sorry* about that, checking now.", field(t, sends[0].Body, "text")["body"])

		wc.deliverWA(map[string]any{"statuses": []any{map[string]any{
			"id": "wamid.OUT1", "status": "read", "timestamp": formatUnix(wc.Clock.Now()), "recipient_id": waCustomer,
		}}})
		wc.deliverWA(map[string]any{"statuses": []any{map[string]any{
			"id": "wamid.OUT1", "status": "delivered", "timestamp": formatUnix(wc.Clock.Now()),
			"recipient_id": waCustomer,
		}}})

		msgs := replies(wc.messages(conv))
		require.Len(t, msgs, 2)
		require.NotNil(t, msgs[1].DeliveryStatus)
		assert.Equal(t, "read", *msgs[1].DeliveryStatus)
	})

	t.Run("after 24 hours only a template goes out", func(t *testing.T) {
		t.Parallel()
		wc := newWhatsAppCase(t)
		wc.deliverWA(customerText("hello", wc.Clock.Now()))
		conv := wc.conversations()[0]
		convID := models.PublicID(models.PrefixConversation, conv.ID)
		wc.Clock.Travel(25 * time.Hour)

		wc.ActingAsOwner().PostJSON("/api/v1/conversations/"+convID+"/replies", map[string]any{"body": "still there?"}).
			AssertStatus(422).AssertJSONPath("code", "channel_window_closed")

		wc.ActingAsOwner().PostJSON("/api/v1/conversations/"+convID+"/whatsapp-templates", map[string]any{
			"name": "order_update", "language": "en_US", "values": map[string]any{"body.1": "Ada"},
		}).AssertStatus(422)

		wc.ActingAsOwner().PostJSON("/api/v1/conversations/"+convID+"/whatsapp-templates", map[string]any{
			"name": "order_update", "language": "en_US", "values": map[string]any{"body.1": "Ada", "body.2": "#42"},
		}).AssertCreated()
		wc.Drain()

		sends := wc.graph.sends()
		require.Len(t, sends, 1)
		assert.Equal(t, "template", sends[0].Body["type"])
		tpl := field(t, sends[0].Body, "template")
		assert.Equal(t, "order_update", tpl["name"])

		msgs := replies(wc.messages(conv))
		assert.Equal(t, "Hi Ada, order #42 shipped.", body(msgs[len(msgs)-1]))
	})

	t.Run("a template Meta refuses is asked of Meta again and fails with what Meta says now", func(t *testing.T) {
		t.Parallel()
		wc := newWhatsAppCase(t)
		wc.deliverWA(customerText("hello", wc.Clock.Now()))
		conv := wc.conversations()[0]
		convID := models.PublicID(models.PrefixConversation, conv.ID)
		wc.Clock.Travel(25 * time.Hour)
		wc.graph.mu.Lock()
		wc.graph.refuseTpl = true
		wc.graph.mu.Unlock()

		wc.ActingAsOwner().PostJSON("/api/v1/conversations/"+convID+"/whatsapp-templates", map[string]any{
			"name": "order_update", "language": "en_US", "values": map[string]any{"body.1": "Ada", "body.2": "#42"},
		}).AssertCreated()
		wc.Drain()

		msgs := replies(wc.messages(conv))
		last := msgs[len(msgs)-1]
		require.NotNil(t, last.DeliveryStatus)
		assert.Equal(t, "failed", *last.DeliveryStatus)
		require.NotNil(t, last.DeliveryError)
		assert.Contains(t, *last.DeliveryError, "Meta paused order_update")

		var status string
		require.NoError(t, wc.Operator().QueryRow(t.Context(),
			"SELECT status FROM whatsapp_templates WHERE channel_connection_id = $1 AND name = 'order_update'",
			wc.conn.ID).Scan(&status))
		assert.Equal(t, "paused", status, "dimmi's copy is what Meta says now")
	})

	t.Run("disconnecting frees the number and keeps its conversations", func(t *testing.T) {
		t.Parallel()
		wc := newWhatsAppCase(t)
		wc.deliverWA(customerText("hello", wc.Clock.Now()))

		wc.ActingAsOwner().Delete("/api/v1/channels/whatsapp/" + wc.publicID()).AssertStatus(204)
		assert.Len(t, wc.conversations(), 1)
		wc.ActingAsOwner().GetJSON("/api/v1/channels/whatsapp").AssertOK().AssertJSONPath("data", []any{})
	})
}
