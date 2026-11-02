package ses_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/mail/drivers/ses"
	"github.com/dimmihq/dimmi/framework/mail/drivertest"
	"github.com/dimmihq/dimmi/framework/mail/mime"
)

/*
request is the JSON body SES receives.
*/
type request struct {
	Content              struct{ Raw struct{ Data []byte } }
	Destination          *struct{ ToAddresses, CcAddresses, BccAddresses []string }
	ConfigurationSetName string
	TenantName           string
	EmailTags            []struct{ Name, Value string }
}

func section(region string) config.Section {
	return config.Section{
		"driver": "ses", "region": region, "access_key_id": "AKID", "secret_access_key": "secret",
		"configuration_set": "system",
	}
}

func newMailer(t *testing.T, region string, resp httpclient.Response) (mail.Mailer, *httpclienttest.Fake) {
	t.Helper()

	client, err := httpclient.New(httpclient.Config{AllowFakes: true})
	require.NoError(t, err)
	fake := httpclienttest.New(client).PreventStrayRequests().
		Stub("POST https://email."+region+".amazonaws.com/v2/email/outbound-emails", resp)

	mailer, err := ses.Factory(t.Context(), client, clock.System())("system", section(region))
	require.NoError(t, err)

	return mailer, fake
}

func accepted() httpclient.Response {
	return httpclient.Response{
		Header: http.Header{"Content-Type": {"application/json"}},
		Body:   []byte(`{"MessageId":"0100018f-abc"}`),
	}
}

/*
sent returns the request whose destination or document carried the
recipient.
*/
func sent(t *testing.T, fake *httpclienttest.Fake, recipient string) request {
	t.Helper()

	var body request
	fake.AssertSent(t, func(req httpclienttest.SentRequest) bool {
		body = request{}
		require.NoError(t, json.Unmarshal(req.Body, &body))

		if body.Destination != nil {
			to := body.Destination

			return slices.Contains(slices.Concat(to.ToAddresses, to.CcAddresses, to.BccAddresses), recipient)
		}

		return bytes.Contains(body.Content.Raw.Data, []byte(recipient))
	})

	return body
}

func newHarness(t *testing.T) drivertest.Harness {
	t.Helper()

	mailer, fake := newMailer(t, "us-east-1", accepted())

	return drivertest.Harness{
		Mailer: mailer,
		Received: func(t *testing.T, recipient string) mail.Message {
			t.Helper()

			msg, err := mime.Decompose(mail.Message{Raw: sent(t, fake, recipient).Content.Raw.Data})
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

	t.Run("sends the stream's configuration set, tenant and tags", func(t *testing.T) {
		t.Parallel()
		mailer, fake := newMailer(t, "us-east-1", accepted())
		to := drivertest.Address("Ana")
		msg := drivertest.Message(to)
		msg.Bcc = []mail.Address{{Email: "copy@example.com"}}
		msg.ConfigurationSet = "customer"
		msg.TenantName = "tenant-42"
		msg.Tags = map[string]string{"stream": "customer", "kind": "reply"}

		result, err := mailer.Send(t.Context(), msg)
		require.NoError(t, err)

		body := sent(t, fake, to.Email)
		assert.Equal(t, "customer", body.ConfigurationSetName)
		assert.Equal(t, "tenant-42", body.TenantName)
		assert.Equal(t, []struct{ Name, Value string }{{"kind", "reply"}, {"stream", "customer"}}, body.EmailTags)
		assert.Equal(t, []string{"copy@example.com"}, body.Destination.BccAddresses)
		assert.Equal(t, "0100018f-abc", result.ProviderMessageID)
		assert.Equal(t, "0100018f-abc@email.amazonses.com", result.MessageID)
	})

	t.Run("uses the configured set and the regional message id domain", func(t *testing.T) {
		t.Parallel()
		mailer, fake := newMailer(t, "eu-west-1", accepted())
		to := drivertest.Address("Ana")

		result, err := mailer.Send(t.Context(), drivertest.Message(to))
		require.NoError(t, err)

		assert.Equal(t, "system", sent(t, fake, to.Email).ConfigurationSetName)
		assert.Equal(t, "0100018f-abc@eu-west-1.amazonses.com", result.MessageID)
	})

	t.Run("rejects a malformed section", func(t *testing.T) {
		t.Parallel()
		client, err := httpclient.New(httpclient.Config{AllowFakes: true})
		require.NoError(t, err)

		_, err = ses.Factory(t.Context(), client, clock.System())("x", config.Section{"region": []int{1}})
		require.Error(t, err)

		_, err = ses.Factory(t.Context(), client, clock.System())("x", config.Section{"configuration_set": []int{1}})
		require.Error(t, err)
	})
}

func refusal(code string) httpclient.Response {
	return httpclient.Response{
		Status: http.StatusBadRequest,
		Header: http.Header{"X-Amzn-Errortype": {code}, "Content-Type": {"application/json"}},
		Body:   []byte(`{"message":"refused"}`),
	}
}

func TestErrors(t *testing.T) {
	t.Parallel()

	answers := map[string]struct {
		resp httpclient.Response
		kind error
	}{
		"refuses a rejected message for good":       {refusal("MessageRejected"), mail.ErrPermanent},
		"refuses an unverified mail from for good":  {refusal("MailFromDomainNotVerifiedException"), mail.ErrPermanent},
		"tries later after a server error":          {httpclient.Response{Status: 502}, mail.ErrTemporary},
		"tries later on an error ses does not name": {refusal("SomethingNewException"), mail.ErrTemporary},
	}

	for name, answer := range answers {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			mailer, _ := newMailer(t, "us-east-1", answer.resp)

			_, err := mailer.Send(t.Context(), drivertest.Message(drivertest.Address("Ana")))

			require.ErrorIs(t, err, answer.kind)
		})
	}

	t.Run("slows down when throttled", func(t *testing.T) {
		t.Parallel()
		mailer, _ := newMailer(t, "us-east-1", refusal("TooManyRequestsException"))

		_, err := mailer.Send(t.Context(), drivertest.Message(drivertest.Address("Ana")))

		var limited *mail.RateLimitedError
		require.ErrorAs(t, err, &limited)
	})

	t.Run("tries later when the request never leaves", func(t *testing.T) {
		t.Parallel()
		mailer, _ := newMailer(t, "us-east-1", accepted())
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		_, err := mailer.Send(ctx, drivertest.Message(drivertest.Address("Ana")))

		require.ErrorIs(t, err, context.Canceled)
		require.ErrorIs(t, err, mail.ErrTemporary)
	})
}
