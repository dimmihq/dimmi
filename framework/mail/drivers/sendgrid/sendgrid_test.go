package sendgrid_test

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
	"github.com/dimmihq/dimmi/framework/mail/drivers/sendgrid"
	"github.com/dimmihq/dimmi/framework/mail/drivertest"
)

const endpoint = "POST https://api.sendgrid.com/v3/mail/send"

/*
attachment is one attachment in the JSON body SendGrid receives.
*/
type attachment struct {
	Filename    string `json:"filename"`
	Content     string `json:"content"`
	Type        string `json:"type"`
	Disposition string `json:"disposition"`
	ContentID   string `json:"content_id"`
}

/*
request is the JSON body SendGrid receives.
*/
type request struct {
	Personalizations []struct{ To, Cc, Bcc []mail.Address }
	From             mail.Address
	ReplyTo          *mail.Address `json:"reply_to"`
	Subject          string        `json:"subject"`
	Content          []struct{ Type, Value string }
	Headers          map[string]string          `json:"headers"`
	CustomArgs       map[string]string          `json:"custom_args"`
	Attachments      []attachment               `json:"attachments"`
	TrackingSettings map[string]map[string]bool `json:"tracking_settings"`
}

func newMailer(t *testing.T, resp httpclient.Response) (mail.Mailer, *httpclienttest.Fake) {
	t.Helper()

	client, err := httpclient.New(httpclient.Config{AllowFakes: true})
	require.NoError(t, err)
	fake := httpclienttest.New(client).PreventStrayRequests().Stub(endpoint, resp)

	mailer, err := sendgrid.Factory(client)("system", config.Section{"api_key": "SG.key"})
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

func received(t *testing.T, body request) mail.Message {
	t.Helper()

	require.Len(t, body.Personalizations, 1)
	msg := mail.Message{
		From: body.From, To: body.Personalizations[0].To, Cc: body.Personalizations[0].Cc,
		Subject: body.Subject, Headers: body.Headers,
	}

	if body.ReplyTo != nil {
		msg.ReplyTo = *body.ReplyTo
	}

	for _, part := range body.Content {
		if part.Type == "text/plain" {
			msg.Text = part.Value
		} else {
			msg.HTML = part.Value
		}
	}

	for _, file := range body.Attachments {
		content, err := base64.StdEncoding.DecodeString(file.Content)
		require.NoError(t, err)

		msg.Attachments = append(msg.Attachments, mail.Attachment{
			Filename: file.Filename, ContentType: file.Type, Content: content,
			Inline: file.Disposition == "inline", ContentID: file.ContentID,
		})
	}

	return msg
}

func newHarness(t *testing.T) drivertest.Harness {
	t.Helper()

	mailer, fake := newMailer(t, httpclient.Response{
		Status: http.StatusAccepted, Header: http.Header{"X-Message-Id": {"sg-1"}},
	})

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

	t.Run("posts with the key, tags as arguments and tracking off", func(t *testing.T) {
		t.Parallel()
		mailer, fake := newMailer(t, httpclient.Response{
			Status: http.StatusAccepted, Header: http.Header{"X-Message-Id": {"sg-1"}},
		})
		to := drivertest.Address("Ana")
		msg := drivertest.Message(to)
		msg.Tags = map[string]string{"stream": "customer"}

		result, err := mailer.Send(t.Context(), msg)
		require.NoError(t, err)

		req, body := sent(t, fake, to.Email)
		assert.Equal(t, "Bearer SG.key", req.Header.Get("Authorization"))
		assert.Equal(t, map[string]string{"stream": "customer"}, body.CustomArgs)
		assert.False(t, body.TrackingSettings["click_tracking"]["enable"])
		assert.False(t, body.TrackingSettings["open_tracking"]["enable"])
		assert.Equal(t, "sg-1", result.ProviderMessageID)
	})

	t.Run("leaves empty optional fields out", func(t *testing.T) {
		t.Parallel()
		mailer, fake := newMailer(t, httpclient.Response{Status: http.StatusAccepted})
		to := drivertest.Address("Ana")

		_, err := mailer.Send(t.Context(), drivertest.Message(to))
		require.NoError(t, err)

		req, _ := sent(t, fake, to.Email)
		assert.NotContains(t, string(req.Body), `"attachments"`)
		assert.NotContains(t, string(req.Body), `"headers"`)
		assert.NotContains(t, string(req.Body), `"custom_args"`)
	})

	t.Run("rejects a malformed section", func(t *testing.T) {
		t.Parallel()

		_, err := sendgrid.Factory(nil)("system", config.Section{"api_key": []int{1}})

		require.Error(t, err)
	})
}

func TestErrors(t *testing.T) {
	t.Parallel()

	t.Run("refuses a bad request for good", func(t *testing.T) {
		t.Parallel()
		mailer, _ := newMailer(t, httpclient.Response{Status: http.StatusBadRequest})

		_, err := mailer.Send(t.Context(), drivertest.Message(drivertest.Address("Ana")))

		require.ErrorIs(t, err, mail.ErrPermanent)
	})
}
