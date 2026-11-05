package gmail_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
	"github.com/dimmihq/dimmi/framework/mailbox"
	"github.com/dimmihq/dimmi/framework/mailbox/drivers/gmail"
	"github.com/dimmihq/dimmi/framework/mailbox/drivertest"
)

const (
	base   = "https://gmail.test"
	me     = base + "/gmail/v1/users/me"
	revoke = "https://oauth.test/revoke"
)

func session() mailbox.Session {
	return mailbox.Session{Token: "tok", Mailbox: "support@acme.test"}
}

func newAccount(t *testing.T) (*gmail.Account, *httpclienttest.Fake) {
	t.Helper()

	client, err := httpclient.New(httpclient.Config{AllowFakes: true})
	require.NoError(t, err)

	fake := httpclienttest.New(client).PreventStrayRequests()

	return gmail.New(client, gmail.Options{ReceivedLabel: "Label_received", BaseURL: base, RevokeURL: revoke}), fake
}

func answer(t *testing.T, value any) httpclient.Response {
	t.Helper()

	body, err := json.Marshal(value)
	require.NoError(t, err)

	return httpclient.Response{Status: http.StatusOK, Body: body}
}

func labeled(t *testing.T, fake *httpclienttest.Fake, id string, labels ...string) {
	t.Helper()

	fake.Stub("GET "+me+"/messages/"+id+"?format=minimal", answer(t, map[string]any{
		"id": id, "threadId": "th-" + id, "labelIds": labels,
	}))
}

func history(ids ...string) map[string]any {
	added := []map[string]any{}
	for _, id := range ids {
		added = append(added, map[string]any{"message": map[string]string{"id": id}})
	}

	return map[string]any{"history": []map[string]any{{"messagesAdded": added}}, "historyId": "200"}
}

func cursorAt(id string) []byte {
	return []byte(`{"history_id":"` + id + `"}`)
}

func harness(t *testing.T) drivertest.AccountHarness {
	t.Helper()

	account, fake := newAccount(t)

	return drivertest.AccountHarness{
		Account: account,
		Session: session(),
		Serve: func(t *testing.T, received, sent string) []byte {
			t.Helper()
			fake.Stub("GET "+me+"/history*", answer(t, history(received, sent)))
			labeled(t, fake, received, "Label_received")
			labeled(t, fake, sent, "SENT")

			return cursorAt("100")
		},
		Stale: func(t *testing.T) []byte {
			t.Helper()
			fake.Stub("GET "+me+"/history*", httpclient.Response{Status: http.StatusNotFound})

			return cursorAt("100")
		},
		Revoked: func(t *testing.T) []byte {
			t.Helper()
			fake.Stub("GET "+me+"/history*", httpclient.Response{Status: http.StatusUnauthorized})

			return cursorAt("100")
		},
		Limited: func(t *testing.T) []byte {
			t.Helper()
			fake.Stub("GET "+me+"/history*", httpclient.Response{
				Status: http.StatusForbidden, Header: http.Header{"Retry-After": {"30"}},
				Body: []byte(`{"error":{"errors":[{"reason":"userRateLimitExceeded"}]}}`),
			})

			return cursorAt("100")
		},
		Raw: func(t *testing.T, id string, raw []byte) {
			t.Helper()
			fake.Stub("GET "+me+"/messages/"+id+"?format=raw", answer(t, map[string]string{
				"raw": base64.URLEncoding.EncodeToString(raw),
			}))
		},
	}
}

func TestAccount(t *testing.T) {
	t.Parallel()

	drivertest.RunAccount(t, harness)
}

func TestProfile(t *testing.T) {
	t.Parallel()

	t.Run("lists only accepted aliases", func(t *testing.T) {
		t.Parallel()
		account, fake := newAccount(t)
		fake.Stub("GET "+me+"/profile", answer(t, map[string]string{"emailAddress": "sam@acme.test"})).
			Stub("GET "+me+"/settings/sendAs", answer(t, map[string]any{"sendAs": []map[string]any{
				{"sendAsEmail": "sam@acme.test", "isPrimary": true},
				{"sendAsEmail": "support@acme.test", "verificationStatus": "accepted"},
				{"sendAsEmail": "pending@acme.test", "verificationStatus": "pending"},
			}}))

		profile, err := account.Profile(t.Context(), session())

		require.NoError(t, err)
		assert.Equal(t, "sam@acme.test", profile.Email)
		assert.Equal(t, []string{"sam@acme.test", "support@acme.test"}, profile.Aliases)
		assert.Equal(t, "gmail", account.Name())
	})

	t.Run("reports a profile it cannot read", func(t *testing.T) {
		t.Parallel()
		account, fake := newAccount(t)
		fake.Stub("GET "+me+"/profile", httpclient.Response{Status: http.StatusServiceUnavailable})

		_, err := account.Profile(t.Context(), session())

		require.Error(t, err)
	})
}

func TestChanges(t *testing.T) {
	t.Parallel()

	t.Run("starts at the mailbox's current history", func(t *testing.T) {
		t.Parallel()
		account, fake := newAccount(t)
		fake.Stub("GET "+me+"/profile", answer(t, map[string]string{"historyId": "555"}))

		cursor, err := account.Cursor(t.Context(), session())

		require.NoError(t, err)
		assert.JSONEq(t, `{"history_id":"555"}`, string(cursor))
	})

	t.Run("skips spam, trash, drafts, chats and anything outside the two folders", func(t *testing.T) {
		t.Parallel()
		account, fake := newAccount(t)
		fake.Stub("GET "+me+"/history*", answer(t, history("a", "b", "c", "d", "e", "f")))
		labeled(t, fake, "a", "Label_received", "SPAM")
		labeled(t, fake, "b", "Label_received", "TRASH")
		labeled(t, fake, "c", "DRAFT")
		labeled(t, fake, "d", "CHATS")
		labeled(t, fake, "e", "CATEGORY_UPDATES")
		labeled(t, fake, "f", "Label_received")

		page, err := account.Changes(t.Context(), session(), cursorAt("1"))

		require.NoError(t, err)
		require.Len(t, page.Items, 1)
		assert.Equal(t, "f", page.Items[0].ID)
		assert.JSONEq(t, `{"history_id":"200"}`, string(page.Cursor))
	})

	t.Run("skips a message deleted before it was read", func(t *testing.T) {
		t.Parallel()
		account, fake := newAccount(t)
		fake.Stub("GET "+me+"/history*", answer(t, history("gone"))).
			Stub("GET "+me+"/messages/gone?format=minimal", httpclient.Response{Status: http.StatusNotFound})

		page, err := account.Changes(t.Context(), session(), cursorAt("1"))

		require.NoError(t, err)
		assert.Empty(t, page.Items)
	})

	t.Run("keeps its place while pages remain", func(t *testing.T) {
		t.Parallel()
		account, fake := newAccount(t)
		page := history("a")
		page["nextPageToken"] = "p2"
		fake.Stub("GET "+me+"/history*", answer(t, page))
		labeled(t, fake, "a", "Label_received")

		first, err := account.Changes(t.Context(), session(), cursorAt("1"))

		require.NoError(t, err)
		assert.True(t, first.More)
		assert.JSONEq(t, `{"history_id":"1","latest":"200","page_token":"p2"}`, string(first.Cursor))

		_, err = account.Changes(t.Context(), session(), first.Cursor)
		require.NoError(t, err)
		fake.AssertSent(t, func(req httpclienttest.SentRequest) bool {
			return strings.Contains(req.URL.RawQuery, "pageToken=p2")
		})
	})

	t.Run("lists recent mail after a cursor went stale", func(t *testing.T) {
		t.Parallel()
		account, fake := newAccount(t)
		fake.Stub("GET "+me+"/profile", answer(t, map[string]string{"historyId": "900"})).
			Stub("GET "+me+"/messages?*", answer(t, map[string]any{"messages": []map[string]string{{"id": "a"}}}))
		labeled(t, fake, "a", "Label_received")

		page, err := account.Since(t.Context(), session(), time.Unix(1_800_000_000, 0))

		require.NoError(t, err)
		require.Len(t, page.Items, 1)
		assert.JSONEq(t, `{"history_id":"900"}`, string(page.Cursor))
		fake.AssertSent(t, func(req httpclienttest.SentRequest) bool {
			return strings.Contains(req.URL.RawQuery, "after%3A1799913600")
		})
	})

	t.Run("reports a failure reading labels or lists", func(t *testing.T) {
		t.Parallel()
		account, fake := newAccount(t)
		fake.Stub("GET "+me+"/history*", answer(t, history("a"))).
			Stub("GET "+me+"/messages/a?format=minimal", httpclient.Response{Status: http.StatusInternalServerError}).
			Stub("GET "+me+"/profile", answer(t, map[string]string{"historyId": "900"})).
			Stub("GET "+me+"/messages?*", httpclient.Response{Status: http.StatusInternalServerError})

		_, err := account.Changes(t.Context(), session(), cursorAt("1"))
		require.Error(t, err)

		_, err = account.Since(t.Context(), session(), time.Unix(1, 0))
		require.Error(t, err)
	})

	t.Run("treats a mailbox lost to its license as out of reach", func(t *testing.T) {
		t.Parallel()
		account, fake := newAccount(t)
		fake.Stub("GET "+me+"/history*", httpclient.Response{
			Status: http.StatusBadRequest, Body: []byte(`{"error":{"errors":[{"reason":"failedPrecondition"}]}}`),
		})

		_, err := account.Changes(t.Context(), session(), cursorAt("1"))

		require.ErrorIs(t, err, mailbox.ErrAccountGone)
	})
}

func TestVerdicts(t *testing.T) {
	t.Parallel()

	t.Run("believes only the top header and only from google", func(t *testing.T) {
		t.Parallel()
		account, _ := newAccount(t)

		trusted := account.Verdicts(
			[]byte("Authentication-Results: mx.google.com; dkim=pass; spf=fail; dmarc=pass\r\n\r\nhi"),
		)
		assert.Equal(t, mailbox.VerdictPass, trusted.DKIM)
		assert.Equal(t, mailbox.VerdictFail, trusted.SPF)
		assert.Equal(t, mailbox.VerdictPass, trusted.DMARC)

		forged := account.Verdicts([]byte("Authentication-Results: evil.test; dkim=pass\r\n" +
			"Authentication-Results: mx.google.com; dkim=fail\r\n\r\nhi"))
		assert.Equal(t, mailbox.Verdicts{}, forged)
	})
}

func TestSend(t *testing.T) {
	t.Parallel()

	send := "POST " + base + "/upload/gmail/v1/users/me/messages/send?uploadType=multipart"
	raw := []byte("From: support@acme.test\r\n\r\nHello\r\n")

	t.Run("sends the raw message in its thread and reads back the id", func(t *testing.T) {
		t.Parallel()
		account, fake := newAccount(t)
		fake.Stub(send, answer(t, map[string]string{"id": "m9", "threadId": "t9"})).
			Stub("GET "+me+"/messages/m9?*", answer(t, map[string]any{"payload": map[string]any{
				"headers": []map[string]string{{"name": "Message-Id", "value": "<real@mail.gmail.test>"}},
			}}))

		sent, err := account.Send(t.Context(), session(), mailbox.SendInput{Raw: raw, ThreadID: "t9"})

		require.NoError(t, err)
		assert.Equal(t, mailbox.Sent{ProviderID: "m9", ThreadID: "t9", RFCMessageID: "real@mail.gmail.test"}, sent)

		fake.AssertSent(t, func(req httpclienttest.SentRequest) bool {
			if req.Method != http.MethodPost {
				return false
			}

			_, params, err := mime.ParseMediaType(req.Header.Get("Content-Type"))
			require.NoError(t, err)

			reader := multipart.NewReader(strings.NewReader(string(req.Body)), params["boundary"])
			first, err := reader.NextPart()
			require.NoError(t, err)
			metadata, err := io.ReadAll(first)
			require.NoError(t, err)
			second, err := reader.NextPart()
			require.NoError(t, err)
			message, err := io.ReadAll(second)
			require.NoError(t, err)

			return strings.Contains(string(metadata), `"threadId":"t9"`) && bytes.Equal(message, raw)
		})
	})

	t.Run("leaves the id to sent sync when it cannot be read", func(t *testing.T) {
		t.Parallel()
		account, fake := newAccount(t)
		fake.Stub(send, answer(t, map[string]string{"id": "m9"})).
			Stub("GET "+me+"/messages/m9?*", httpclient.Response{Status: http.StatusInternalServerError})

		sent, err := account.Send(t.Context(), session(), mailbox.SendInput{Raw: raw})

		require.NoError(t, err)
		assert.Empty(t, sent.RFCMessageID)
	})

	t.Run("maps gmail's answers", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			status int
			body   string
			check  func(t *testing.T, err error)
		}{
			{http.StatusForbidden, `{"error":{"errors":[{"reason":"domainPolicy"}]}}`, func(t *testing.T, err error) {
				t.Helper()
				require.ErrorIs(t, err, mailbox.ErrSendRefused)
			}},
			{http.StatusTooManyRequests, `{}`, func(t *testing.T, err error) {
				t.Helper()
				_, ok := errors.AsType[*mailbox.RateLimitedError](err)
				assert.True(t, ok)
			}},
			{http.StatusUnauthorized, `{}`, func(t *testing.T, err error) {
				t.Helper()
				require.ErrorIs(t, err, mailbox.ErrGrantRevoked)
			}},
			{http.StatusInternalServerError, `not json`, func(t *testing.T, err error) {
				t.Helper()
				require.ErrorContains(t, err, "500")
			}},
		}

		for _, c := range cases {
			account, fake := newAccount(t)
			fake.Stub(send, httpclient.Response{Status: c.status, Body: []byte(c.body)})

			_, err := account.Send(t.Context(), session(), mailbox.SendInput{Raw: raw})
			c.check(t, err)
		}
	})
}

func TestWatch(t *testing.T) {
	t.Parallel()

	t.Run("asks for both labels on the topic", func(t *testing.T) {
		t.Parallel()
		account, fake := newAccount(t)
		fake.Stub("POST "+me+"/watch", answer(t, map[string]string{"historyId": "1", "expiration": "1800000000000"}))

		watching, err := account.Watch(t.Context(), session(), mailbox.WatchInput{Topic: "projects/p/topics/t"})

		require.NoError(t, err)
		assert.Equal(t, time.UnixMilli(1_800_000_000_000), watching.ExpiresAt)
		fake.AssertSent(t, func(req httpclienttest.SentRequest) bool {
			return strings.Contains(string(req.Body), `"labelIds":["Label_received","SENT"]`)
		})
	})

	t.Run("reports an expiry it cannot read and a refusal", func(t *testing.T) {
		t.Parallel()
		account, fake := newAccount(t)
		fake.Stub("POST "+me+"/watch", answer(t, map[string]string{"expiration": "soon"}))

		_, err := account.Watch(t.Context(), session(), mailbox.WatchInput{})
		require.Error(t, err)

		refused, refusedFake := newAccount(t)
		refusedFake.Stub("POST "+me+"/watch", httpclient.Response{Status: http.StatusForbidden})

		_, err = refused.Watch(t.Context(), session(), mailbox.WatchInput{})
		require.ErrorIs(t, err, mailbox.ErrAccountGone)
	})

	t.Run("stops the watch", func(t *testing.T) {
		t.Parallel()
		account, fake := newAccount(t)
		fake.Stub("POST "+me+"/stop", httpclient.Response{Status: http.StatusNoContent})

		require.NoError(t, account.StopWatch(t.Context(), session(), nil))
	})

	t.Run("revokes the grant at google", func(t *testing.T) {
		t.Parallel()
		account, fake := newAccount(t)
		fake.Stub("POST "+revoke+"?token=refresh", httpclient.Response{Status: http.StatusOK})

		require.NoError(t, account.Revoke(t.Context(), "refresh"))
	})

	t.Run("reports a revoke google refuses", func(t *testing.T) {
		t.Parallel()
		account, fake := newAccount(t)
		fake.Stub("POST "+revoke+"*", httpclient.Response{Status: http.StatusBadRequest})

		require.Error(t, account.Revoke(t.Context(), "refresh"))
	})
}

func TestUnreachable(t *testing.T) {
	t.Parallel()

	t.Run("reports every call that cannot reach google", func(t *testing.T) {
		t.Parallel()
		account, _ := newAccount(t)
		ctx := t.Context()

		_, err := account.Profile(ctx, session())
		require.Error(t, err)
		_, err = account.Cursor(ctx, session())
		require.Error(t, err)
		_, err = account.Since(ctx, session(), time.Unix(1, 0))
		require.Error(t, err)
		_, err = account.Raw(ctx, session(), "m-1")
		require.Error(t, err)
		_, err = account.Send(ctx, session(), mailbox.SendInput{Raw: []byte("hi")})
		require.Error(t, err)
		_, err = account.Watch(ctx, session(), mailbox.WatchInput{})
		require.Error(t, err)
		require.Error(t, account.StopWatch(ctx, session(), nil))
		require.Error(t, account.Revoke(ctx, "refresh"))
	})

	t.Run("reports aliases it cannot read", func(t *testing.T) {
		t.Parallel()
		account, fake := newAccount(t)
		fake.Stub("GET "+me+"/profile", answer(t, map[string]string{"emailAddress": "support@acme.test"}))

		_, err := account.Profile(t.Context(), session())

		require.Error(t, err)
	})

	t.Run("reports answers it cannot decode", func(t *testing.T) {
		t.Parallel()
		account, fake := newAccount(t)
		fake.Stub("GET "+me+"/messages/m-1?format=raw", answer(t, map[string]string{"raw": "%%%"})).
			Stub("POST "+base+"/upload/gmail/v1/users/me/messages/send*",
				httpclient.Response{Status: http.StatusOK, Body: []byte("nope")}).
			Stub("POST "+me+"/watch", httpclient.Response{Status: http.StatusOK, Body: []byte("nope")})

		_, err := account.Raw(t.Context(), session(), "m-1")
		require.Error(t, err)
		_, err = account.Send(t.Context(), session(), mailbox.SendInput{Raw: []byte("hi")})
		require.Error(t, err)
		_, err = account.Watch(t.Context(), session(), mailbox.WatchInput{})
		require.Error(t, err)
	})

	t.Run("reports a page of labels it cannot read after listing", func(t *testing.T) {
		t.Parallel()
		account, fake := newAccount(t)
		fake.Stub("GET "+me+"/profile", answer(t, map[string]string{"historyId": "900"})).
			Stub("GET "+me+"/messages?*", answer(t, map[string]any{"messages": []map[string]string{{"id": "a"}}}))

		_, err := account.Since(t.Context(), session(), time.Unix(1, 0))

		require.Error(t, err)
	})
}
