package postmark_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	netmail "net/mail"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/mail/drivers/postmark"
	"github.com/dimmihq/dimmi/framework/mail/drivertest"
)

const endpoint = "POST https://api.postmarkapp.com/email"

/*
request is the JSON body Postmark receives.
*/
type request struct {
	From, To, Cc, Bcc, ReplyTo, Subject, TextBody, HTMLBody string
	MessageStream, TrackLinks                               string
	TrackOpens                                              bool
	Metadata                                                map[string]string
	Headers                                                 []struct{ Name, Value string }
	Attachments                                             []struct{ Name, Content, ContentType, ContentID string }
}

func newMailer(t *testing.T, resp httpclient.Response) (mail.Mailer, *httpclienttest.Fake) {
	t.Helper()

	client, err := httpclient.New(httpclient.Config{AllowFakes: true})
	require.NoError(t, err)
	fake := httpclienttest.New(client).PreventStrayRequests().Stub(endpoint, resp)

	mailer, err := postmark.Factory(client)("system", config.Section{"token": "server-token"})
	require.NoError(t, err)

	return mailer, fake
}

/*
sent returns the request that carried the recipient.
*/
func sent(t *testing.T, fake *httpclienttest.Fake, recipient string) (httpclienttest.SentRequest, request) {
	t.Helper()

	var found httpclienttest.SentRequest
	fake.AssertSent(t, func(req httpclienttest.SentRequest) bool {
		found = req

		return bytes.Contains(req.Body, []byte(recipient))
	})

	var body request
	require.NoError(t, json.Unmarshal(found.Body, &body))

	return found, body
}

func addresses(t *testing.T, list string) []mail.Address {
	t.Helper()

	if list == "" {
		return nil
	}

	parsed, err := netmail.ParseAddressList(list)
	require.NoError(t, err)

	out := make([]mail.Address, 0, len(parsed))
	for _, a := range parsed {
		out = append(out, mail.Address{Name: a.Name, Email: a.Address})
	}

	return out
}

func first(list []mail.Address) mail.Address {
	if len(list) == 0 {
		return mail.Address{}
	}

	return list[0]
}

/*
received reads the request back into the message Postmark will send.
*/
func received(t *testing.T, body request) mail.Message {
	t.Helper()

	msg := mail.Message{
		From:    first(addresses(t, body.From)),
		ReplyTo: first(addresses(t, body.ReplyTo)),
		To:      addresses(t, body.To),
		Cc:      addresses(t, body.Cc),
		Subject: body.Subject,
		Text:    body.TextBody,
		HTML:    body.HTMLBody,
		Headers: map[string]string{},
	}

	for _, header := range body.Headers {
		msg.Headers[header.Name] = header.Value
	}

	for _, file := range body.Attachments {
		content, err := base64.StdEncoding.DecodeString(file.Content)
		require.NoError(t, err)

		msg.Attachments = append(msg.Attachments, mail.Attachment{
			Filename: file.Name, ContentType: file.ContentType, Content: content,
			Inline: file.ContentID != "", ContentID: strings.TrimPrefix(file.ContentID, "cid:"),
		})
	}

	return msg
}

func newHarness(t *testing.T) drivertest.Harness {
	t.Helper()

	mailer, fake := newMailer(t, httpclient.Response{Body: []byte(`{"MessageID":"pm-1","ErrorCode":0}`)})

	return drivertest.Harness{
		Mailer: mailer,
		Received: func(t *testing.T, recipient string) mail.Message {
			t.Helper()

			_, body := sent(t, fake, recipient)

			return received(t, body)
		},
	}
}

func TestMailer(t *testing.T) {
	t.Parallel()

	drivertest.Run(t, newHarness)
}

func TestRequest(t *testing.T) {
	t.Parallel()

	t.Run("posts with the token, stream and tracking off", func(t *testing.T) {
		t.Parallel()
		mailer, fake := newMailer(t, httpclient.Response{Body: []byte(`{"MessageID":"pm-1"}`)})
		to := drivertest.Address("Ana")
		msg := drivertest.Message(to)
		msg.Tags = map[string]string{"stream": "customer"}

		result, err := mailer.Send(t.Context(), msg)
		require.NoError(t, err)

		req, body := sent(t, fake, to.Email)
		assert.Equal(t, "server-token", req.Header.Get("X-Postmark-Server-Token"))
		assert.Equal(t, "outbound", body.MessageStream)
		assert.False(t, body.TrackOpens)
		assert.Equal(t, "None", body.TrackLinks)
		assert.Equal(t, map[string]string{"stream": "customer"}, body.Metadata)
		assert.Equal(t, "pm-1", result.ProviderMessageID)
	})

	t.Run("rejects a malformed section", func(t *testing.T) {
		t.Parallel()

		_, err := postmark.Factory(nil)("system", config.Section{"token": []int{1}})

		require.Error(t, err)
	})
}

func TestErrors(t *testing.T) {
	t.Parallel()

	answers := map[string]struct {
		resp httpclient.Response
		kind error
	}{
		"refuses an inactive recipient for good": {
			httpclient.Response{Status: http.StatusUnprocessableEntity, Body: []byte(`{"ErrorCode":406}`)},
			mail.ErrPermanent,
		},
		"tries later after a server error": {
			httpclient.Response{Status: http.StatusInternalServerError}, mail.ErrTemporary,
		},
		"tries later when the token is refused": {
			httpclient.Response{Status: http.StatusUnauthorized}, mail.ErrTemporary,
		},
	}

	for name, answer := range answers {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			mailer, _ := newMailer(t, answer.resp)

			_, err := mailer.Send(t.Context(), drivertest.Message(drivertest.Address("Ana")))

			require.ErrorIs(t, err, answer.kind)
		})
	}

	t.Run("slows down when rate limited", func(t *testing.T) {
		t.Parallel()
		mailer, _ := newMailer(t, httpclient.Response{
			Status: http.StatusTooManyRequests, Header: http.Header{"Retry-After": {"20"}},
		})

		_, err := mailer.Send(t.Context(), drivertest.Message(drivertest.Address("Ana")))

		var limited *mail.RateLimitedError
		require.ErrorAs(t, err, &limited)
		assert.Equal(t, 20*time.Second, limited.RetryAfter)
	})

	t.Run("reports an answer that is not json", func(t *testing.T) {
		t.Parallel()
		mailer, _ := newMailer(t, httpclient.Response{Body: []byte("ok")})

		_, err := mailer.Send(t.Context(), drivertest.Message(drivertest.Address("Ana")))

		require.Error(t, err)
	})
}
