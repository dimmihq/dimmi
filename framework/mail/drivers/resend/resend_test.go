package resend_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/mail/drivers/resend"
	"github.com/dimmihq/dimmi/framework/mail/drivertest"
)

const endpoint = "POST https://api.resend.com/emails"

/*
attachment is one attachment in the JSON body Resend receives.
*/
type attachment struct {
	Filename    string `json:"filename"`
	Content     string `json:"content"`
	ContentType string `json:"content_type"`
	ContentID   string `json:"content_id"`
}

/*
request is the JSON body Resend receives.
*/
type request struct {
	From        string            `json:"from"`
	To          []string          `json:"to"`
	Cc          []string          `json:"cc"`
	Bcc         []string          `json:"bcc"`
	ReplyTo     []string          `json:"reply_to"`
	Subject     string            `json:"subject"`
	Text        string            `json:"text"`
	HTML        string            `json:"html"`
	Headers     map[string]string `json:"headers"`
	Tags        []struct{ Name, Value string }
	Attachments []attachment `json:"attachments"`
}

func newMailer(t *testing.T, resp httpclient.Response) (mail.Mailer, *httpclienttest.Fake) {
	t.Helper()

	client, err := httpclient.New(httpclient.Config{AllowFakes: true})
	require.NoError(t, err)
	fake := httpclienttest.New(client).PreventStrayRequests().Stub(endpoint, resp)

	mailer, err := resend.Factory(client)("system", config.Section{"api_key": "re_key"})
	require.NoError(t, err)

	return mailer, fake
}

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

func addresses(t *testing.T, list []string) []mail.Address {
	t.Helper()

	var out []mail.Address
	for _, text := range list {
		addr, err := mail.ParseAddress(text)
		require.NoError(t, err)
		out = append(out, addr)
	}

	return out
}

func received(t *testing.T, body request) mail.Message {
	t.Helper()

	from, err := mail.ParseAddress(body.From)
	require.NoError(t, err)

	msg := mail.Message{
		From: from, To: addresses(t, body.To), Cc: addresses(t, body.Cc),
		Subject: body.Subject, Text: body.Text, HTML: body.HTML, Headers: body.Headers,
	}

	if replyTo := addresses(t, body.ReplyTo); len(replyTo) > 0 {
		msg.ReplyTo = replyTo[0]
	}

	for _, file := range body.Attachments {
		content, err := base64.StdEncoding.DecodeString(file.Content)
		require.NoError(t, err)

		msg.Attachments = append(msg.Attachments, mail.Attachment{
			Filename: file.Filename, ContentType: file.ContentType, Content: content,
			Inline: file.ContentID != "", ContentID: file.ContentID,
		})
	}

	return msg
}

func newHarness(t *testing.T) drivertest.Harness {
	t.Helper()

	mailer, fake := newMailer(t, httpclient.Response{Body: []byte(`{"id":"re-1"}`)})

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

	t.Run("posts with the key and sorted tags", func(t *testing.T) {
		t.Parallel()
		mailer, fake := newMailer(t, httpclient.Response{Body: []byte(`{"id":"re-1"}`)})
		to := drivertest.Address("Ana")
		msg := drivertest.Message(to)
		msg.Tags = map[string]string{"stream": "customer", "kind": "reply"}

		result, err := mailer.Send(t.Context(), msg)
		require.NoError(t, err)

		req, body := sent(t, fake, to.Email)
		assert.Equal(t, "Bearer re_key", req.Header.Get("Authorization"))
		assert.Equal(t, []struct{ Name, Value string }{{"kind", "reply"}, {"stream", "customer"}}, body.Tags)
		assert.Equal(t, "re-1", result.ProviderMessageID)
	})

	t.Run("rejects a malformed section", func(t *testing.T) {
		t.Parallel()

		_, err := resend.Factory(nil)("system", config.Section{"api_key": []int{1}})

		require.Error(t, err)
	})
}

func TestErrors(t *testing.T) {
	t.Parallel()

	t.Run("refuses a validation error for good", func(t *testing.T) {
		t.Parallel()
		mailer, _ := newMailer(t, httpclient.Response{Status: http.StatusUnprocessableEntity})

		_, err := mailer.Send(t.Context(), drivertest.Message(drivertest.Address("Ana")))

		require.ErrorIs(t, err, mail.ErrPermanent)
	})

	t.Run("reports an answer that is not json", func(t *testing.T) {
		t.Parallel()
		mailer, _ := newMailer(t, httpclient.Response{Body: []byte("ok")})

		_, err := mailer.Send(t.Context(), drivertest.Message(drivertest.Address("Ana")))

		require.Error(t, err)
	})
}
