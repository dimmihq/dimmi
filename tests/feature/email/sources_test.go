package email_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
	"github.com/dimmihq/dimmi/framework/mailbox/drivers/cloudflare"
	"github.com/dimmihq/dimmi/framework/mailbox/drivers/mailgun"
	"github.com/dimmihq/dimmi/framework/mailbox/drivers/resend"
	"github.com/dimmihq/dimmi/framework/mailbox/drivers/sendgrid"
	"github.com/dimmihq/dimmi/framework/mailbox/drivers/sendgrid/sendgridtest"
	"github.com/dimmihq/dimmi/framework/testkit"
	"github.com/dimmihq/dimmi/framework/webhooks"

	"github.com/dimmihq/dimmi/tests"
)

const (
	mailgunKey       = "mailgun-signing-key"
	cloudflareSecret = "cloudflare-shared-secret"
	resendAPI        = "https://api.resend.test"
	mailgunRefused   = 406
)

/*
sourceCase boots the application with the named source enabled and its
options set.
*/
func sourceCase(t *testing.T, source string, options map[string]string) *tests.TestCase {
	t.Helper()

	env := []string{
		"DIMMI_CHANNELS__EMAIL__INBOUND_DOMAIN=" + inboundDomain,
		"DIMMI_CHANNELS__EMAIL__INBOUND__DRIVERS=" + source,
	}
	for key, value := range options {
		env = append(env, fmt.Sprintf("DIMMI_MAILBOX__SOURCES__%s__%s=%s", envName(source), key, value))
	}

	return tests.New(t, tests.WithEnv(env...))
}

func envName(source string) string {
	return string(bytes.ToUpper([]byte(source)))
}

/*
inboundRaw returns a raw email to the workspace's forwarding address.
*/
func inboundRaw(tc *tests.TestCase) ([]byte, string) {
	tc.T.Helper()

	address := defaultAddress(tc).ForwardingAddress
	id := strconv.FormatInt(tc.Clock.Now().UnixNano(), 10)

	return []byte("From: Bob <bob@customer.test>\r\nTo: " + address + "\r\nSubject: Refund for order 1042\r\n" +
		"Message-ID: <" + id + "@customer.test>\r\nDate: Mon, 2 Nov 2026 10:00:00 +0000\r\n\r\n" +
		"Charged twice.\r\n"), address
}

/*
assertBecameConversation checks that the mail reached the inbox.
*/
func assertBecameConversation(tc *tests.TestCase, source string) {
	tc.T.Helper()
	tc.Drain()

	found := conversations(tc)
	require.Len(tc.T, found, 1)
	assert.Equal(tc.T, "Refund for order 1042", *found[0].Subject)

	rows := inboundEmails(tc, defaultAddress(tc))
	require.Len(tc.T, rows, 1)
	assert.Equal(tc.T, source, rows[0].Source)
}

func postmarkPost(t *testing.T, raw []byte, address string) []byte {
	t.Helper()

	body, err := json.Marshal(map[string]any{
		"MessageID": "pm-1", "OriginalRecipient": address, "RawEmail": string(raw),
		"Headers": []map[string]string{{"Name": "Return-Path", "Value": "<bob@customer.test>"}},
	})
	require.NoError(t, err)

	return body
}

func basicAuth(password string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte("hook:"+password))
}

func TestPostmarkSource(t *testing.T) {
	t.Parallel()

	options := map[string]string{"USERNAME": "hook", "PASSWORD": "secret"}
	path := "/webhooks/postmark/inbound"

	t.Run("turns a post into a conversation", func(t *testing.T) {
		t.Parallel()
		tc := sourceCase(t, "postmark", options)
		raw, address := inboundRaw(tc)

		tc.Client().WithHeader("Authorization", basicAuth("secret")).
			Send("POST", path, postmarkPost(t, raw, address)).AssertOK()

		assertBecameConversation(tc, "postmark")
	})

	t.Run("records one message when postmark delivers twice", func(t *testing.T) {
		t.Parallel()
		tc := sourceCase(t, "postmark", options)
		raw, address := inboundRaw(tc)
		client := tc.Client().WithHeader("Authorization", basicAuth("secret"))

		client.Send("POST", path, postmarkPost(t, raw, address)).AssertOK()
		client.Send("POST", path, postmarkPost(t, raw, address)).AssertOK()

		assertBecameConversation(tc, "postmark")
	})

	t.Run("tells postmark to stop on bad credentials", func(t *testing.T) {
		t.Parallel()
		tc := sourceCase(t, "postmark", options)

		tc.Client().WithHeader("Authorization", basicAuth("wrong")).
			Send("POST", path, []byte("{}")).AssertForbidden()
	})

	t.Run("keeps the mail at postmark and names the setting when raw is off", func(t *testing.T) {
		t.Parallel()
		tc := sourceCase(t, "postmark", options)
		_, address := inboundRaw(tc)

		body, err := json.Marshal(map[string]any{"MessageID": "pm-2", "OriginalRecipient": address})
		require.NoError(t, err)

		client := tc.Client().WithHeader("Authorization", basicAuth("secret"))
		client.Send("POST", path, body).AssertStatus(fhttp.StatusServiceUnavailable)

		setting, err := tc.App.Actions.InboundHealth.Misconfigured(t.Context(), "postmark")
		require.NoError(t, err)
		assert.Contains(t, setting, "raw email content")

		raw, _ := inboundRaw(tc)
		client.Send("POST", path, postmarkPost(t, raw, address)).AssertOK()

		setting, err = tc.App.Actions.InboundHealth.Misconfigured(t.Context(), "postmark")
		require.NoError(t, err)
		assert.Empty(t, setting)
	})
}

func mailgunPost(tc *tests.TestCase, raw []byte, address, token string) []byte {
	tc.T.Helper()

	timestamp := strconv.FormatInt(tc.Clock.Now().Unix(), 10)

	return []byte(url.Values{
		"timestamp": {timestamp}, "token": {token}, "signature": {mailgun.Sign(mailgunKey, timestamp, token)},
		"recipient": {address}, "sender": {"bob@customer.test"}, "body-mime": {string(raw)},
	}.Encode())
}

func TestMailgunSource(t *testing.T) {
	t.Parallel()

	options := map[string]string{"SIGNING_KEY": mailgunKey}
	path := "/webhooks/mailgun/inbound/mime"

	t.Run("turns a post into a conversation", func(t *testing.T) {
		t.Parallel()
		tc := sourceCase(t, "mailgun", options)
		raw, address := inboundRaw(tc)

		tc.Client().WithHeader("Content-Type", "application/x-www-form-urlencoded").
			Send("POST", path, mailgunPost(tc, raw, address, "token-1")).AssertOK()

		assertBecameConversation(tc, "mailgun")
	})

	t.Run("refuses a token already accepted and tells mailgun to stop", func(t *testing.T) {
		t.Parallel()
		tc := sourceCase(t, "mailgun", options)
		raw, address := inboundRaw(tc)
		body := mailgunPost(tc, raw, address, "token-2")
		client := tc.Client().WithHeader("Content-Type", "application/x-www-form-urlencoded")

		client.Send("POST", path, body).AssertOK()
		client.Send("POST", path, body).AssertStatus(mailgunRefused)
	})

	t.Run("lets a post whose raw message could not be kept come back", func(t *testing.T) {
		t.Parallel()
		tc := sourceCase(t, "mailgun", options)
		raw, address := inboundRaw(tc)
		body := mailgunPost(tc, raw, address, "token-3")
		client := tc.Client().WithHeader("Content-Type", "application/x-www-form-urlencoded")

		tc.BreakTable("inbound_emails")
		client.Send("POST", path, body).AssertStatus(fhttp.StatusServiceUnavailable)
	})

	t.Run("refuses to work while the replay window cannot be checked", func(t *testing.T) {
		t.Parallel()
		tc := sourceCase(t, "mailgun", options)
		raw, address := inboundRaw(tc)
		body := mailgunPost(tc, raw, address, "token-4")

		tc.BreakValkey()
		tc.Client().WithHeader("Content-Type", "application/x-www-form-urlencoded").
			Send("POST", path, body).AssertStatus(fhttp.StatusServiceUnavailable)
	})
}

func TestSendGridSource(t *testing.T) {
	t.Parallel()

	signer := sendgridtest.NewSigner(t)
	options := map[string]string{"PUBLIC_KEY": signer.PublicKey()}
	path := "/webhooks/sendgrid/inbound"

	post := func(t *testing.T, tc *tests.TestCase, raw []byte, address string) *testkit.Response {
		t.Helper()

		signed := signer.Sign(t, strconv.FormatInt(tc.Clock.Now().Unix(), 10), map[string]string{
			"email": string(raw), "envelope": `{"to":["` + address + `"],"from":"bob@customer.test"}`, "SPF": "pass",
		})

		return tc.Client().WithHeader("Content-Type", signed.ContentType).
			WithHeader(sendgrid.HeaderTimestamp, signed.Timestamp).
			WithHeader(sendgrid.HeaderSignature, signed.Signature).
			Send("POST", path, signed.Body)
	}

	t.Run("turns a post into a conversation", func(t *testing.T) {
		t.Parallel()
		tc := sourceCase(t, "sendgrid", options)
		raw, address := inboundRaw(tc)

		post(t, tc, raw, address).AssertOK()

		assertBecameConversation(tc, "sendgrid")
	})

	t.Run("refuses an unsigned post", func(t *testing.T) {
		t.Parallel()
		tc := sourceCase(t, "sendgrid", options)

		tc.Client().Send("POST", path, []byte("hello")).AssertUnauthorized()
	})
}

func TestResendSource(t *testing.T) {
	t.Parallel()

	secret := webhooks.NewSecret()
	options := map[string]string{"WEBHOOK_SECRET": secret.Encode(), "API_KEY": "re_test", "BASE_URL": resendAPI}
	path := "/webhooks/resend/inbound"

	post := func(t *testing.T, tc *tests.TestCase, address string) *testkit.Response {
		t.Helper()

		body, err := json.Marshal(map[string]any{
			"type": "email.received",
			"data": map[string]any{"email_id": "em-1", "from": "bob@customer.test", "to": []string{address}},
		})
		require.NoError(t, err)

		return tc.Client().WithHeader(resend.HeaderID, "msg_1").
			WithHeader(resend.HeaderTimestamp, strconv.FormatInt(tc.Clock.Now().Unix(), 10)).
			WithHeader(resend.HeaderSignature, webhooks.Sign(secret, "msg_1", tc.Clock.Now(), body)).
			Send("POST", path, body)
	}

	t.Run("fetches the raw message and turns it into a conversation", func(t *testing.T) {
		t.Parallel()
		tc := sourceCase(t, "resend", options)
		raw, address := inboundRaw(tc)

		link, err := json.Marshal(map[string]any{"raw": map[string]string{"download_url": resendAPI + "/raw/em-1"}})
		require.NoError(t, err)
		httpclienttest.New(tc.App.HTTP).PreventStrayRequests().
			Stub("GET "+resendAPI+"/emails/receiving/em-1", httpclient.Response{Status: fhttp.StatusOK, Body: link}).
			Stub("GET "+resendAPI+"/raw/em-1", httpclient.Response{Status: fhttp.StatusOK, Body: raw})

		post(t, tc, address).AssertOK()

		assertBecameConversation(tc, "resend")
	})

	t.Run("asks for a retry while the raw message is not ready", func(t *testing.T) {
		t.Parallel()
		tc := sourceCase(t, "resend", options)
		_, address := inboundRaw(tc)

		httpclienttest.New(tc.App.HTTP).PreventStrayRequests().
			Stub("GET "+resendAPI+"/emails/receiving/em-1", httpclient.Response{
				Status: fhttp.StatusOK, Body: []byte(`{"raw":null}`),
			})

		post(t, tc, address).AssertStatus(fhttp.StatusServiceUnavailable)
	})
}

func TestCloudflareSource(t *testing.T) {
	t.Parallel()

	options := map[string]string{"SECRET": cloudflareSecret}
	path := "/webhooks/cloudflare/inbound"

	post := func(tc *tests.TestCase, raw []byte, address, secret string, at int64) *testkit.Response {
		timestamp := strconv.FormatInt(at, 10)

		return tc.Client().WithHeader("Content-Type", "message/rfc822").
			WithHeader(cloudflare.HeaderTimestamp, timestamp).
			WithHeader(cloudflare.HeaderSignature, cloudflare.Sign(secret, timestamp, raw)).
			WithHeader(cloudflare.HeaderEnvelopeFrom, "bob@customer.test").
			WithHeader(cloudflare.HeaderEnvelopeTo, address).
			Send("POST", path, raw)
	}

	t.Run("turns a post into a conversation", func(t *testing.T) {
		t.Parallel()
		tc := sourceCase(t, "cloudflare", options)
		raw, address := inboundRaw(tc)

		post(tc, raw, address, cloudflareSecret, tc.Clock.Now().Unix()).AssertOK()

		assertBecameConversation(tc, "cloudflare")
	})

	t.Run("names the missing fallback address the worker reports", func(t *testing.T) {
		t.Parallel()
		tc := sourceCase(t, "cloudflare", options)
		raw, address := inboundRaw(tc)
		timestamp := strconv.FormatInt(tc.Clock.Now().Unix(), 10)

		tc.Client().WithHeader("Content-Type", "message/rfc822").
			WithHeader(cloudflare.HeaderTimestamp, timestamp).
			WithHeader(cloudflare.HeaderSignature, cloudflare.Sign(cloudflareSecret, timestamp, raw)).
			WithHeader(cloudflare.HeaderEnvelopeTo, address).
			WithHeader(cloudflare.HeaderFallback, "off").
			Send("POST", path, raw).AssertOK()

		setting, err := tc.App.Actions.InboundHealth.Misconfigured(t.Context(), "cloudflare")
		require.NoError(t, err)
		assert.Equal(t, cloudflare.FallbackAdvice, setting)
	})

	t.Run("refuses a bad signature and an old one", func(t *testing.T) {
		t.Parallel()
		tc := sourceCase(t, "cloudflare", options)
		raw, address := inboundRaw(tc)

		post(tc, raw, address, "wrong", tc.Clock.Now().Unix()).AssertUnauthorized()
		post(tc, raw, address, cloudflareSecret, tc.Clock.Now().Unix()-600).AssertUnauthorized()
	})

	t.Run("refuses a body over the limit", func(t *testing.T) {
		t.Parallel()
		tc := sourceCase(t, "cloudflare", map[string]string{"SECRET": cloudflareSecret, "MAX_WEBHOOK_BYTES": "32"})
		raw, address := inboundRaw(tc)

		post(tc, raw, address, cloudflareSecret, tc.Clock.Now().Unix()).
			AssertStatus(fhttp.StatusRequestEntityTooLarge)
	})
}

func TestSourceRoutes(t *testing.T) {
	t.Parallel()

	t.Run("answers not found for a source the install did not enable", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)

		for _, path := range []string{
			"/webhooks/postmark/inbound", "/webhooks/mailgun/inbound/mime", "/webhooks/sendgrid/inbound",
			"/webhooks/resend/inbound", "/webhooks/cloudflare/inbound",
		} {
			tc.Client().Send("POST", path, []byte("{}")).AssertNotFound().
				AssertJSONPath("code", "inbound_driver_disabled")
		}
	})

	t.Run("refuses to boot an enabled source without its secret and names the key", func(t *testing.T) {
		t.Parallel()

		_, _, err := tests.Boot(t, tests.WithEnv(
			"DIMMI_CHANNELS__EMAIL__INBOUND_DOMAIN="+inboundDomain,
			"DIMMI_CHANNELS__EMAIL__INBOUND__DRIVERS=mailgun",
			"DIMMI_MAILBOX__SOURCES__MAILGUN__DRIVER=mailgun",
		))

		require.ErrorContains(t, err, "mailbox.sources.mailgun.signing_key")
	})
}
