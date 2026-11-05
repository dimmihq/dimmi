package graph_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
	"github.com/dimmihq/dimmi/framework/mailbox"
	"github.com/dimmihq/dimmi/framework/mailbox/drivers/graph"
	"github.com/dimmihq/dimmi/framework/mailbox/drivertest"
)

const (
	base = "https://graph.test"
	user = base + "/users/support@acme.test"
)

func session() mailbox.Session {
	return mailbox.Session{Token: "tok", Mailbox: "support@acme.test"}
}

func now() time.Time {
	return time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC)
}

func newAccount(t *testing.T) (*graph.Account, *httpclienttest.Fake) {
	t.Helper()

	client, err := httpclient.New(httpclient.Config{AllowFakes: true})
	require.NoError(t, err)

	fake := httpclienttest.New(client).PreventStrayRequests()

	return graph.New(client, clock.NewFake(now()), graph.Options{ReceivedFolder: "received", BaseURL: base}), fake
}

func answer(t *testing.T, value any) httpclient.Response {
	t.Helper()

	body, err := json.Marshal(value)
	require.NoError(t, err)

	return httpclient.Response{Status: http.StatusOK, Body: body}
}

func deltaPage(next, delta string, ids ...string) map[string]any {
	values := []map[string]string{}
	for _, id := range ids {
		values = append(values, map[string]string{"id": id, "internetMessageId": "<" + id + "@x>"})
	}

	page := map[string]any{"value": values}
	if next != "" {
		page["@odata.nextLink"] = next
	}

	if delta != "" {
		page["@odata.deltaLink"] = delta
	}

	return page
}

func at(received, sent string) []byte {
	body, _ := json.Marshal(map[string]string{"received_delta": received, "sent_delta": sent})

	return body
}

func harness(t *testing.T) drivertest.AccountHarness {
	t.Helper()

	account, fake := newAccount(t)
	in, out := base+"/in", base+"/out"

	return drivertest.AccountHarness{
		Account: account,
		Session: session(),
		Serve: func(t *testing.T, received, sent string) []byte {
			t.Helper()
			fake.Stub("GET "+out, answer(t, deltaPage("", base+"/out2", sent)))
			fake.Stub("GET "+in, answer(t, deltaPage("", base+"/in2", received)))

			return at(in, out)
		},
		Stale: func(t *testing.T) []byte {
			t.Helper()
			fake.Stub("GET "+out, httpclient.Response{Status: http.StatusGone})

			return at(in, out)
		},
		Revoked: func(t *testing.T) []byte {
			t.Helper()
			fake.Stub("GET "+out, httpclient.Response{Status: http.StatusUnauthorized})

			return at(in, out)
		},
		Limited: func(t *testing.T) []byte {
			t.Helper()
			fake.Stub("GET "+out, httpclient.Response{
				Status: http.StatusTooManyRequests, Header: http.Header{"Retry-After": {"30"}},
			})

			return at(in, out)
		},
		Raw: func(t *testing.T, id string, raw []byte) {
			t.Helper()
			fake.Stub("GET "+user+"/messages/"+id+"/$value", httpclient.Response{Status: http.StatusOK, Body: raw})
		},
	}
}

func TestAccount(t *testing.T) {
	t.Parallel()

	drivertest.RunAccount(t, harness)
}

func TestProfile(t *testing.T) {
	t.Parallel()

	t.Run("names the signed-in account, by its login when it has no mail", func(t *testing.T) {
		t.Parallel()
		account, fake := newAccount(t)
		fake.Stub("GET "+base+"/me", answer(t, map[string]string{"mail": "sam@acme.test"}))

		profile, err := account.Profile(t.Context(), session())
		require.NoError(t, err)
		assert.Equal(t, "sam@acme.test", profile.Email)
		assert.Equal(t, "graph", account.Name())

		bare, bareFake := newAccount(t)
		bareFake.Stub("GET "+base+"/me", answer(t, map[string]string{"userPrincipalName": "sam@live.test"}))

		profile, err = bare.Profile(t.Context(), session())
		require.NoError(t, err)
		assert.Equal(t, "sam@live.test", profile.Email)
	})

	t.Run("reports a failure", func(t *testing.T) {
		t.Parallel()
		account, fake := newAccount(t)
		fake.Stub("GET "+base+"/me", httpclient.Response{Status: http.StatusBadGateway})

		_, err := account.Profile(t.Context(), session())

		require.ErrorContains(t, err, "502")
	})
}

func TestChanges(t *testing.T) {
	t.Parallel()

	t.Run("starts each feed at this moment and asks for immutable ids", func(t *testing.T) {
		t.Parallel()
		account, fake := newAccount(t)
		fake.Stub("GET "+user+"/mailFolders/received/messages/delta*", answer(t, deltaPage(base+"/in-next", "")))
		fake.Stub("GET "+base+"/in-next", answer(t, deltaPage("", base+"/in-delta")))
		fake.Stub("GET "+user+"/mailFolders/sentitems/messages/delta*", answer(t, deltaPage("", base+"/out-delta")))

		cursor, err := account.Cursor(t.Context(), session())

		require.NoError(t, err)
		assert.JSONEq(t, string(at(base+"/in-delta", base+"/out-delta")), string(cursor))
		fake.AssertSent(t, func(req httpclienttest.SentRequest) bool {
			return req.Header.Get("Prefer") == `IdType="ImmutableId"` &&
				strings.Contains(req.URL.RawQuery, "2026-11-04T10%3A00%3A00Z")
		})
	})

	t.Run("reports a feed that never settles or fails to start", func(t *testing.T) {
		t.Parallel()
		account, fake := newAccount(t)
		fake.Stub("GET "+user+"/mailFolders/received/messages/delta*", answer(t, deltaPage("", "")))

		_, err := account.Cursor(t.Context(), session())
		require.ErrorIs(t, err, mailbox.ErrCursorExpired)

		failing, failingFake := newAccount(t)
		failingFake.Stub("GET "+user+"/mailFolders/received/messages/delta*", answer(t, deltaPage("", base+"/in"))).
			Stub("GET "+user+"/mailFolders/sentitems/messages/delta*",
				httpclient.Response{Status: http.StatusForbidden})

		_, err = failing.Cursor(t.Context(), session())
		require.ErrorIs(t, err, mailbox.ErrAccountGone)
	})

	t.Run("waits with received mail until sent mail has caught up", func(t *testing.T) {
		t.Parallel()
		account, fake := newAccount(t)
		fake.Stub("GET "+base+"/out", answer(t, deltaPage(base+"/out-next", "", "s1")))

		page, err := account.Changes(t.Context(), session(), at(base+"/in", base+"/out"))

		require.NoError(t, err)
		assert.True(t, page.More)
		require.Len(t, page.Items, 1)
		assert.Equal(t, "s1@x", page.Items[0].RFCMessageID)
		assert.Empty(t, page.Items[0].ThreadID)
		assert.JSONEq(t, string(at(base+"/in", base+"/out-next")), string(page.Cursor))
	})

	t.Run("passes over messages removed from a folder", func(t *testing.T) {
		t.Parallel()
		account, fake := newAccount(t)
		fake.Stub("GET "+base+"/out", answer(t, deltaPage("", base+"/out2")))
		fake.Stub("GET "+base+"/in", answer(t, map[string]any{
			"value":            []map[string]any{{"id": "x", "@removed": map[string]string{"reason": "deleted"}}},
			"@odata.deltaLink": base + "/in2",
		}))

		page, err := account.Changes(t.Context(), session(), at(base+"/in", base+"/out"))

		require.NoError(t, err)
		assert.Empty(t, page.Items)
		assert.False(t, page.More)
	})

	t.Run("reports a received feed that fails after sent mail caught up", func(t *testing.T) {
		t.Parallel()
		account, fake := newAccount(t)
		fake.Stub("GET "+base+"/out", answer(t, deltaPage("", base+"/out2")))
		fake.Stub("GET "+base+"/in", httpclient.Response{
			Status: http.StatusBadRequest, Body: []byte(`{"error":{"code":"SyncStateNotFound"}}`),
		})

		_, err := account.Changes(t.Context(), session(), at(base+"/in", base+"/out"))

		require.ErrorIs(t, err, mailbox.ErrCursorExpired)
	})

	t.Run("lists recent mail after a cursor went stale", func(t *testing.T) {
		t.Parallel()
		account, fake := newAccount(t)
		fake.Stub("GET "+user+"/mailFolders/received/messages/delta*", answer(t, deltaPage("", base+"/in"))).
			Stub("GET "+user+"/mailFolders/sentitems/messages/delta*", answer(t, deltaPage("", base+"/out"))).
			Stub("GET "+user+"/mailFolders/sentitems/messages?*", answer(t, deltaPage("", "", "s1"))).
			Stub("GET "+user+"/mailFolders/received/messages?*", answer(t, deltaPage("", "", "r1")))

		page, err := account.Since(t.Context(), session(), now().Add(-time.Hour))

		require.NoError(t, err)
		require.Len(t, page.Items, 2)
		assert.Equal(t, mailbox.FolderSent, page.Items[0].Folder)
		assert.JSONEq(t, string(at(base+"/in", base+"/out")), string(page.Cursor))
	})

	t.Run("reports a failed list and a failed raw read", func(t *testing.T) {
		t.Parallel()
		account, fake := newAccount(t)
		fake.Stub("GET "+user+"/mailFolders/received/messages/delta*",
			httpclient.Response{Status: http.StatusBadGateway})
		fake.Stub("GET "+user+"/messages/m/$value", httpclient.Response{Status: http.StatusNotFound})

		_, err := account.Since(t.Context(), session(), now())
		require.Error(t, err)

		_, err = account.Raw(t.Context(), session(), "m")
		require.ErrorIs(t, err, mailbox.ErrAccountGone)
	})
}

func TestVerdicts(t *testing.T) {
	t.Parallel()

	t.Run("believes only the top header", func(t *testing.T) {
		t.Parallel()
		account, _ := newAccount(t)

		verdicts := account.Verdicts([]byte("Authentication-Results: spf.protection.outlook.com; dkim=pass\r\n" +
			"Authentication-Results: evil.test; dkim=fail; dmarc=pass\r\n\r\nhi"))

		assert.Equal(t, mailbox.Verdicts{DKIM: mailbox.VerdictPass}, verdicts)
	})
}

func TestSend(t *testing.T) {
	t.Parallel()

	t.Run("posts the raw message as base64 text", func(t *testing.T) {
		t.Parallel()
		account, fake := newAccount(t)
		fake.Stub("POST "+user+"/sendMail", httpclient.Response{Status: http.StatusAccepted})

		sent, err := account.Send(t.Context(), session(), mailbox.SendInput{Raw: []byte("Hi")})

		require.NoError(t, err)
		assert.Equal(t, mailbox.Sent{}, sent)
		fake.AssertSent(t, func(req httpclienttest.SentRequest) bool {
			return string(req.Body) == "SGk=" && req.Header.Get("Content-Type") == "text/plain"
		})
	})

	t.Run("tells a permanent refusal from a retry", func(t *testing.T) {
		t.Parallel()
		account, fake := newAccount(t)
		fake.Stub("POST "+user+"/sendMail", httpclient.Response{
			Status: http.StatusBadRequest, Body: []byte(`{"error":{"code":"ErrorSendAsDenied"}}`),
		})

		_, err := account.Send(t.Context(), session(), mailbox.SendInput{Raw: []byte("Hi")})
		require.ErrorIs(t, err, mailbox.ErrSendRefused)

		retried, retriedFake := newAccount(t)
		retriedFake.Stub("POST "+user+"/sendMail",
			httpclient.Response{Status: http.StatusBadGateway, Body: []byte("oops")})

		_, err = retried.Send(t.Context(), session(), mailbox.SendInput{Raw: []byte("Hi")})
		require.Error(t, err)
		assert.NotErrorIs(t, err, mailbox.ErrSendRefused)
	})
}

func TestSubscriptions(t *testing.T) {
	t.Parallel()

	input := mailbox.WatchInput{
		NotificationURL: "https://app.test/n", LifecycleURL: "https://app.test/l", ClientState: "state",
	}

	t.Run("subscribes to each folder for six days", func(t *testing.T) {
		t.Parallel()
		account, fake := newAccount(t)
		fake.Stub("POST "+base+"/subscriptions", answer(t, map[string]string{"id": "sub-1"}))

		watching, err := account.Watch(t.Context(), session(), input)

		require.NoError(t, err)
		assert.Equal(t, now().Add(6*24*time.Hour), watching.ExpiresAt)
		assert.Contains(t, string(watching.State), "sub-1")
		fake.AssertSent(t, func(req httpclienttest.SentRequest) bool {
			return strings.Contains(string(req.Body), "mailFolders/sentitems/messages") &&
				strings.Contains(string(req.Body), `"clientState":"state"`)
		})
	})

	t.Run("renews a known subscription and recreates one microsoft forgot", func(t *testing.T) {
		t.Parallel()
		account, fake := newAccount(t)
		fake.Stub("PATCH "+base+"/subscriptions/known", answer(t, map[string]string{"id": "known"})).
			Stub("PATCH "+base+"/subscriptions/lost", httpclient.Response{Status: http.StatusNotFound}).
			Stub("POST "+base+"/subscriptions", answer(t, map[string]string{"id": "fresh"}))

		renew := input
		renew.Existing = []byte(`{"received":{"id":"known"},"sent":{"id":"lost"}}`)

		watching, err := account.Watch(t.Context(), session(), renew)

		require.NoError(t, err)
		assert.Contains(t, string(watching.State), "known")
		assert.Contains(t, string(watching.State), "fresh")
	})

	t.Run("treats an existing duplicate as watching", func(t *testing.T) {
		t.Parallel()
		account, fake := newAccount(t)
		fake.Stub("POST "+base+"/subscriptions", httpclient.Response{Status: http.StatusConflict})

		_, err := account.Watch(t.Context(), session(), input)

		require.NoError(t, err)
	})

	t.Run("reports state it cannot read and failures", func(t *testing.T) {
		t.Parallel()
		account, fake := newAccount(t)
		fake.Stub("POST "+base+"/subscriptions", httpclient.Response{Status: http.StatusForbidden}).
			Stub("PATCH "+base+"/subscriptions/x", httpclient.Response{Status: http.StatusBadGateway})

		broken := input
		broken.Existing = []byte("nope")
		_, err := account.Watch(t.Context(), session(), broken)
		require.Error(t, err)

		_, err = account.Watch(t.Context(), session(), input)
		require.ErrorIs(t, err, mailbox.ErrAccountGone)

		renewing := input
		renewing.Existing = []byte(`{"received":{"id":"x"}}`)
		_, err = account.Watch(t.Context(), session(), renewing)
		require.Error(t, err)

		require.Error(t, account.StopWatch(t.Context(), session(), []byte("nope")))
	})

	t.Run("deletes subscriptions, not minding one already gone", func(t *testing.T) {
		t.Parallel()
		account, fake := newAccount(t)
		fake.Stub("DELETE "+base+"/subscriptions/a", httpclient.Response{Status: http.StatusNoContent}).
			Stub("DELETE "+base+"/subscriptions/b", httpclient.Response{Status: http.StatusNotFound})

		state := []byte(`{"received":{"id":"a"},"sent":{"id":"b"}}`)
		require.NoError(t, account.StopWatch(t.Context(), session(), state))

		failing, failingFake := newAccount(t)
		failingFake.Stub("DELETE "+base+"/subscriptions/a", httpclient.Response{Status: http.StatusBadGateway})
		require.Error(t, failing.StopWatch(t.Context(), session(), state))
	})

	t.Run("has no single grant to revoke", func(t *testing.T) {
		t.Parallel()
		account, _ := newAccount(t)

		require.NoError(t, account.Revoke(t.Context(), "refresh"))
	})
}
