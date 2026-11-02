package mailgun_test

import (
	"bytes"
	"encoding/base64"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/mail/drivers/mailgun"
	"github.com/dimmihq/dimmi/framework/mail/drivertest"
	mailmime "github.com/dimmihq/dimmi/framework/mail/mime"
)

const endpoint = "POST https://api.mailgun.net/v3/mg.example.com/messages.mime"

func newMailer(t *testing.T, resp httpclient.Response) (mail.Mailer, *httpclienttest.Fake) {
	t.Helper()

	client, err := httpclient.New(httpclient.Config{AllowFakes: true})
	require.NoError(t, err)
	fake := httpclienttest.New(client).PreventStrayRequests().Stub(endpoint, resp)

	mailer, err := mailgun.Factory(client, clock.System())("system", config.Section{
		"api_key": "key-1", "domain": "mg.example.com",
	})
	require.NoError(t, err)

	return mailer, fake
}

/*
form reads the multipart form of the request that carried the
recipient: its fields, and the message file under "message".
*/
func form(t *testing.T, fake *httpclienttest.Fake, recipient string) (httpclienttest.SentRequest, map[string]string) {
	t.Helper()

	var found httpclienttest.SentRequest
	fake.AssertSent(t, func(req httpclienttest.SentRequest) bool {
		found = req

		return bytes.Contains(req.Body, []byte(recipient))
	})

	_, params, err := mime.ParseMediaType(found.Header.Get("Content-Type"))
	require.NoError(t, err)

	fields := map[string]string{}
	reader := multipart.NewReader(bytes.NewReader(found.Body), params["boundary"])
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoError(t, err)

		value, err := io.ReadAll(part)
		require.NoError(t, err)
		fields[part.FormName()] = string(value)
	}

	return found, fields
}

func newHarness(t *testing.T) drivertest.Harness {
	t.Helper()

	mailer, fake := newMailer(t, httpclient.Response{Body: []byte(`{"id":"<mg-1@mg.example.com>","message":"Queued"}`)})

	return drivertest.Harness{
		Mailer: mailer,
		Received: func(t *testing.T, recipient string) mail.Message {
			t.Helper()

			_, fields := form(t, fake, recipient)
			assert.Contains(t, strings.Split(fields["to"], ","), recipient)

			msg, err := mailmime.Decompose(mail.Message{Raw: []byte(fields["message"])})
			require.NoError(t, err)

			return msg
		},
	}
}

func TestMailer(t *testing.T) {
	t.Parallel()

	drivertest.Run(t, newHarness)
}

func TestRequest(t *testing.T) {
	t.Parallel()

	t.Run("posts with basic auth, tags as variables and tracking off", func(t *testing.T) {
		t.Parallel()
		mailer, fake := newMailer(t, httpclient.Response{Body: []byte(`{"id":"<mg-1@mg.example.com>"}`)})
		to := drivertest.Address("Ana")
		msg := drivertest.Message(to)
		msg.Tags = map[string]string{"stream": "customer"}

		result, err := mailer.Send(t.Context(), msg)
		require.NoError(t, err)

		req, fields := form(t, fake, to.Email)
		basic := "Basic " + base64.StdEncoding.EncodeToString([]byte("api:key-1"))
		assert.Equal(t, basic, req.Header.Get("Authorization"))
		assert.Equal(t, "customer", fields["v:stream"])
		assert.Equal(t, "no", fields["o:tracking"])
		assert.Equal(t, "mg-1@mg.example.com", result.ProviderMessageID)
	})

	t.Run("rejects a malformed section", func(t *testing.T) {
		t.Parallel()

		_, err := mailgun.Factory(nil, clock.System())("system", config.Section{"domain": []int{1}})

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

	t.Run("reports an answer that is not json", func(t *testing.T) {
		t.Parallel()
		mailer, _ := newMailer(t, httpclient.Response{Body: []byte("ok")})

		_, err := mailer.Send(t.Context(), drivertest.Message(drivertest.Address("Ana")))

		require.Error(t, err)
	})

	t.Run("refuses a raw message with broken recipients", func(t *testing.T) {
		t.Parallel()
		mailer, _ := newMailer(t, httpclient.Response{})

		_, err := mailer.Send(t.Context(), mail.Message{Raw: []byte("From: a@example.com\r\nTo: <<x\r\n\r\nbody")})

		require.ErrorIs(t, err, mail.ErrPermanent)
	})
}
