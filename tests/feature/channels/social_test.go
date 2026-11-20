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

	"github.com/dimmihq/dimmi/app/drivers/channel/meta"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

/*
The Meta app, Page and Instagram account the social tests connect.
*/
const (
	metaAppID     = "777001"
	metaSecret    = "meta-app-secret"
	metaVerify    = "meta-verify-token"
	businessToken = "BISU-test-token"
	pageToken     = "PAGE-test-token"
	pageID        = "100200"
	igID          = "17841400"
	psid          = "555001"
	igsid         = "900001"
)

/*
fakeMeta answers the Graph API calls the two channels make and records
them.
*/
type fakeMeta struct {
	mu    sync.Mutex
	calls []graphCall
	sent  int
}

func (f *fakeMeta) serve(w fhttp.ResponseWriter, r *fhttp.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v27.0/")
	call := graphCall{Method: r.Method, Path: path}

	if r.Body != nil {
		raw, _ := io.ReadAll(r.Body)

		switch {
		case strings.HasPrefix(r.Header.Get("Content-Type"), "application/json"):
			_ = json.Unmarshal(raw, &call.Body)
		case strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data"):
			call.Body = map[string]any{"multipart": string(raw)}
		}
	}

	f.mu.Lock()
	f.calls = append(f.calls, call)
	f.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")

	switch {
	case path == "oauth/access_token":
		_, _ = io.WriteString(w, `{"access_token":"`+businessToken+`"}`)
	case path == "me":
		_, _ = io.WriteString(w, `{"client_business_id":"B77","id":"SU1"}`)
	case path == "me/assigned_pages":
		_, _ = io.WriteString(w, `{"data":[{"id":"`+pageID+`","name":"Acme Page","access_token":"`+pageToken+`",`+
			`"picture":{"data":{"url":"https://scontent.xx.fbcdn.net/page.jpg"}}}]}`)
	case path == "me/assigned_instagram_accounts":
		_, _ = io.WriteString(w, `{"data":[{"id":"`+igID+`","username":"acme","name":"Acme",`+
			`"profile_picture_url":"https://scontent.cdninstagram.com/acme.jpg"}]}`)
	case strings.HasSuffix(path, "/subscribed_apps") && r.Method == fhttp.MethodGet:
		_, _ = io.WriteString(w, `{"data":[{"id":"`+metaAppID+`","subscribed_fields":["messages"]}]}`)
	case strings.HasSuffix(path, "/subscribed_apps"):
		_, _ = io.WriteString(w, `{"success":true}`)
	case strings.HasSuffix(path, "/conversations") && r.URL.Query().Get("user_id") == psid:
		_, _ = io.WriteString(w, `{"data":[{"id":"t1","participants":{"data":[`+
			`{"id":"`+psid+`","name":"Peter Chang"},{"id":"`+pageID+`","name":"Acme Page"}]}}]}`)
	case strings.HasSuffix(path, "/conversations"):
		_, _ = io.WriteString(w, `{"data":[{"id":"t1","updated_time":"2026-11-20T16:30:22+0000"}]}`)
	case path == psid:
		w.WriteHeader(fhttp.StatusBadRequest)
		_, _ = io.WriteString(w, `{"error":{"code":100,"message":"Unsupported get request."}}`)
	case path == igsid:
		_, _ = io.WriteString(w, `{"name":"Ada Lovelace","username":"ada","follower_count":1234,`+
			`"is_verified_user":true,"is_user_follow_business":true}`)
	case strings.HasSuffix(path, "/messages") && call.Body["sender_action"] != nil:
		_, _ = io.WriteString(w, `{"recipient_id":"x"}`)
	case strings.HasSuffix(path, "/messages"):
		f.mu.Lock()
		f.sent++
		id := "mid.OUT" + strconv.Itoa(f.sent)
		f.mu.Unlock()

		_, _ = io.WriteString(w, `{"recipient_id":"x","message_id":"`+id+`"}`)
	default:
		w.WriteHeader(fhttp.StatusNotFound)
		_, _ = io.WriteString(w, `{"error":{"code":100,"message":"unknown path"}}`)
	}
}

/*
sends returns the messages a channel sent to customers.
*/
func (f *fakeMeta) sends() []graphCall {
	f.mu.Lock()
	defer f.mu.Unlock()

	var out []graphCall

	for _, c := range f.calls {
		if strings.HasSuffix(c.Path, "/messages") && c.Body["sender_action"] == nil {
			out = append(out, c)
		}
	}

	return out
}

/*
called reports whether the fake received a call to the path.
*/
func (f *fakeMeta) called(method, path string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, c := range f.calls {
		if c.Method == method && c.Path == path {
			return true
		}
	}

	return false
}

/*
socialCase is a test with Meta faked and one platform's account
connected through a sign-in.
*/
type socialCase struct {
	*slackCase
	meta     *fakeMeta
	platform string
	account  string
	customer string
}

func newSocialCase(t *testing.T, platform string) *socialCase {
	t.Helper()

	tc := tests.New(t, tests.WithEnv(
		"DIMMI_SERVICES__META__APP_ID="+metaAppID, "DIMMI_SERVICES__META__APP_SECRET="+metaSecret,
		"DIMMI_SERVICES__META__VERIFY_TOKEN="+metaVerify,
		"DIMMI_SERVICES__FACEBOOK__CONFIG_ID=cfg-fb", "DIMMI_SERVICES__INSTAGRAM__CONFIG_ID=cfg-ig",
	))

	fake := &fakeMeta{}
	httpclienttest.New(tc.App.HTTP).Serve("https://graph.facebook.com/*", fhttp.HandlerFunc(fake.serve))

	sc := &socialCase{
		slackCase: &slackCase{TestCase: tc}, meta: fake, platform: platform, account: pageID, customer: psid,
	}
	if platform == "instagram" {
		sc.account, sc.customer = igID, igsid
	}

	grant := tc.ActingAsOwner().PostJSON("/api/v1/channels/"+platform+"/signups", map[string]any{"code": "c0de"}).
		AssertOK()
	require.Equal(t, sc.account, grant.String("data.accounts.0.id"))
	require.Equal(t, "false", grant.String("data.accounts.0.connected"))

	res := tc.ActingAsOwner().PostJSON("/api/v1/channels/"+platform, map[string]any{
		"grant": grant.String("data.grant"), "account_id": sc.account,
	}).AssertCreated()

	id, err := tc.App.IDs.Decode(string(models.PrefixChannelConnection), res.String("data.id"))
	require.NoError(t, err)

	sc.conn = sc.connection(id)

	return sc
}

/*
deliverEvents posts signed events to the platform's address and
processes them.
*/
func (sc *socialCase) deliverEvents(events ...map[string]any) *testkit.Response {
	sc.T.Helper()

	object := "page"
	if sc.platform == "instagram" {
		object = "instagram"
	}

	raw, _ := json.Marshal(map[string]any{
		"object": object,
		"entry":  []any{map[string]any{"id": sc.account, "time": sc.Clock.Now().UnixMilli(), "messaging": events}},
	})

	res := sc.Client().WithHeader("Content-Type", "application/json").
		WithHeader(meta.SignatureHeader, meta.Sign(metaSecret, raw)).Send("POST", "/webhooks/"+sc.platform, raw)
	sc.Drain()

	return res
}

/*
wrote is the customer's message with the id and what it holds.
*/
func (sc *socialCase) wrote(id string, message map[string]any) map[string]any {
	message["mid"] = id

	return map[string]any{
		"sender": map[string]any{"id": sc.customer}, "recipient": map[string]any{"id": sc.account},
		"timestamp": sc.Clock.Now().UnixMilli(), "message": message,
	}
}

func (sc *socialCase) reply(conv models.Conversation, text string) *testkit.Response {
	convID := models.PublicID(models.PrefixConversation, conv.ID)

	res := sc.ActingAsOwner().PostJSON("/api/v1/conversations/"+convID+"/replies", map[string]any{"body": text})
	sc.Drain()

	return res
}

func TestSocialChannels(t *testing.T) {
	t.Parallel()

	for _, platform := range []string{"facebook", "instagram"} {
		t.Run(platform+" connects the account picked from a sign-in and subscribes it", func(t *testing.T) {
			t.Parallel()

			sc := newSocialCase(t, platform)

			facts := map[string]any{}
			require.NoError(t, json.Unmarshal(sc.conn.Config, &facts))
			assert.Equal(t, "active", sc.conn.Status)
			assert.Equal(t, platform, sc.conn.Type)
			assert.Equal(t, sc.account, facts["account_id"])
			assert.Equal(t, "B77", facts["business_id"])
			assert.True(t, sc.meta.called("POST", sc.account+"/subscribed_apps"))

			list := sc.ActingAsOwner().GetJSON("/api/v1/channels/" + platform).AssertOK()
			assert.Equal(t, sc.account, list.String("data.0.account_id"))
			assert.Equal(t, platform, list.String("data.0.platform"))

			again := sc.ActingAsOwner().PostJSON("/api/v1/channels/"+platform+"/signups",
				map[string]any{"code": "c0de"}).
				AssertOK()
			assert.Equal(t, "true", again.String("data.accounts.0.connected"))
			sc.ActingAsOwner().PostJSON("/api/v1/channels/"+platform, map[string]any{
				"grant": again.String("data.grant"), "account_id": sc.account,
			}).AssertStatus(409)

			sc.ActingAsOwner().PostJSON("/api/v1/channels/"+platform, map[string]any{
				"grant": "nope", "account_id": sc.account,
			}).AssertStatus(422)
		})

		t.Run(platform+" asks Meta what stands between a connection and its messages", func(t *testing.T) {
			t.Parallel()

			sc := newSocialCase(t, platform)

			address := "/api/v1/channels/" + platform + "/" + sc.publicID() + "/check"
			res := sc.ActingAsOwner().GetJSON(address).AssertOK()
			assert.Equal(t, "token", res.String("data.0.check"))
			assert.Equal(t, "true", res.String("data.0.ok"))
			assert.Equal(t, "delivery", res.String("data.1.check"))
			assert.Equal(t, "true", res.String("data.1.ok"))
			assert.Equal(t, "messages", res.String("data.2.check"))
			assert.Contains(t, res.String("data.2.detail"), "2026-11-20")
		})

		t.Run(platform+" answers Meta's address check and refuses unsigned deliveries", func(t *testing.T) {
			t.Parallel()

			sc := newSocialCase(t, platform)

			ok := sc.Client().Send("GET", "/webhooks/"+platform+"?hub.mode=subscribe&hub.verify_token="+metaVerify+
				"&hub.challenge=c123", nil).AssertOK()
			assert.Equal(t, "c123", string(ok.Body))
			sc.Client().Send("GET",
				"/webhooks/"+platform+"?hub.mode=subscribe&hub.verify_token=no&hub.challenge=c1", nil).
				AssertStatus(403)

			raw := []byte(`{"object":"page","entry":[]}`)
			sc.Client().Send("POST", "/webhooks/"+platform, raw).AssertUnauthorized()
		})

		t.Run(platform+" turns a customer's message into a conversation once, and replies to it", func(t *testing.T) {
			t.Parallel()

			sc := newSocialCase(t, platform)

			sc.deliverEvents(sc.wrote("m_1", map[string]any{"text": "Where is my order?"})).AssertOK()
			sc.deliverEvents(sc.wrote("m_1", map[string]any{"text": "Where is my order?"}))

			convs := sc.conversations()
			require.Len(t, convs, 1)
			assert.Equal(t, "dm:"+sc.customer, *convs[0].ExternalThreadKey)

			msgs := sc.messages(convs[0])
			require.Len(t, msgs, 1)
			assert.Equal(t, "Where is my order?", body(msgs[0]))

			convID := models.PublicID(models.PrefixConversation, convs[0].ID)
			state := sc.ActingAsOwner().GetJSON("/api/v1/conversations/" + convID + "/" + platform).AssertOK()
			assert.Equal(t, "open", state.String("data.phase"))
			wantName := map[string]string{"facebook": "Peter Chang", "instagram": "Ada Lovelace"}[platform]
			assert.Equal(t, wantName, state.String("data.profile.name"))

			sc.reply(convs[0], "**On its way**, arriving tomorrow.").AssertCreated()

			sends := sc.meta.sends()
			require.Len(t, sends, 1)
			assert.Equal(t, sc.account+"/messages", sends[0].Path)
			assert.Equal(t, sc.customer, field(t, sends[0].Body, "recipient")["id"])
			assert.Equal(t, "RESPONSE", sends[0].Body["messaging_type"])
			assert.Equal(t, "On its way, arriving tomorrow.", field(t, sends[0].Body, "message")["text"])
		})

		t.Run(platform+" lets a teammate reply for a week with the human-agent tag, then nobody", func(t *testing.T) {
			t.Parallel()

			sc := newSocialCase(t, platform)
			sc.deliverEvents(sc.wrote("m_7", map[string]any{"text": "hello"}))

			conv := sc.conversations()[0]
			convID := models.PublicID(models.PrefixConversation, conv.ID)

			sc.Clock.Travel(25 * time.Hour)

			state := sc.ActingAsOwner().GetJSON("/api/v1/conversations/" + convID + "/" + platform).AssertOK()
			assert.Equal(t, "human", state.String("data.phase"))

			sc.reply(conv, "Sorry for the wait.").AssertCreated()

			sends := sc.meta.sends()
			require.Len(t, sends, 1)
			assert.Equal(t, "MESSAGE_TAG", sends[0].Body["messaging_type"])
			assert.Equal(t, "HUMAN_AGENT", sends[0].Body["tag"])

			sc.Clock.Travel(7 * 24 * time.Hour)

			state = sc.ActingAsOwner().GetJSON("/api/v1/conversations/" + convID + "/" + platform).AssertOK()
			assert.Equal(t, "closed", state.String("data.phase"))
			sc.reply(conv, "Still there?").AssertStatus(422)
			assert.Len(t, sc.meta.sends(), 1)
		})

		t.Run(platform+" follows an edit and an unsend, and skips its own echo", func(t *testing.T) {
			t.Parallel()

			sc := newSocialCase(t, platform)
			sc.deliverEvents(sc.wrote("m_1", map[string]any{"text": "my card is 4242"}))

			conv := sc.conversations()[0]
			sc.reply(conv, "Please do not share card numbers.").AssertCreated()

			sc.deliverEvents(map[string]any{
				"sender": map[string]any{"id": sc.account}, "recipient": map[string]any{"id": sc.customer},
				"timestamp": sc.Clock.Now().UnixMilli(),
				"message": map[string]any{
					"mid":     "mid.OUT1",
					"is_echo": true, "text": "Please do not share card numbers.",
				},
			})
			assert.Len(t, replies(sc.messages(conv)), 2, "the echo of our own reply adds nothing")

			sc.deliverEvents(map[string]any{
				"sender": map[string]any{"id": sc.customer}, "recipient": map[string]any{"id": sc.account},
				"timestamp":    sc.Clock.Now().UnixMilli(),
				"message_edit": map[string]any{"mid": "m_1", "text": "my card is hidden"},
			})
			assert.Equal(t, "my card is hidden", body(replies(sc.messages(conv))[0]))

			sc.deliverEvents(sc.wrote("m_1", map[string]any{"is_deleted": true}))
			unsent := replies(sc.messages(conv))[0]
			assert.Contains(t, string(unsent.Metadata), `"source_deleted"`)
		})
	}
}
