package ses_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/aws"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/mail/drivers/ses"
)

const identitiesBase = "https://email.eu-west-1.amazonaws.com/v2/email"

func newIdentities(t *testing.T) (*ses.Identities, *httpclienttest.Fake) {
	t.Helper()

	client, err := httpclient.New(httpclient.Config{AllowFakes: true})
	require.NoError(t, err)

	cfg, err := aws.Config(t.Context(), section("eu-west-1"), client)
	require.NoError(t, err)

	return ses.NewIdentities(cfg), httpclienttest.New(client).PreventStrayRequests()
}

func jsonResponse(status int, body string, code string) httpclient.Response {
	header := http.Header{"Content-Type": {"application/json"}}
	if code != "" {
		header.Set("X-Amzn-Errortype", code)
	}

	return httpclient.Response{Status: status, Header: header, Body: []byte(body)}
}

func identityBody() string {
	return `{"VerifiedForSendingStatus":true,"DkimAttributes":{"Tokens":["a","b","c"],` +
		`"SigningHostedZone":"dkim.amazonses.com","Status":"SUCCESS"},` +
		`"MailFromAttributes":{"MailFromDomain":"bounce.acme.test","MailFromDomainStatus":"PENDING"}}`
}

func TestIdentities(t *testing.T) {
	t.Parallel()

	t.Run("creates an identity with a long key and returns its tokens", func(t *testing.T) {
		t.Parallel()
		identities, fake := newIdentities(t)
		fake.Stub("POST "+identitiesBase+"/identities", jsonResponse(http.StatusOK, `{}`, "")).
			Stub("GET "+identitiesBase+"/identities/acme.test", jsonResponse(http.StatusOK, identityBody(), ""))

		got, err := identities.Create(t.Context(), "acme.test")

		require.NoError(t, err)
		assert.Equal(t, []string{"a", "b", "c"}, got.Tokens)
		assert.Equal(t, "dkim.amazonses.com", got.SigningHostedZone)
		assert.Equal(t, "SUCCESS", got.DKIMStatus)
		assert.Equal(t, "PENDING", got.MailFromStatus)
		assert.True(t, got.Verified)
		assert.Equal(t, "eu-west-1", identities.Region())
		fake.AssertSent(t, func(req httpclienttest.SentRequest) bool {
			return strings.Contains(string(req.Body), "RSA_2048_BIT")
		})
	})

	t.Run("treats an identity that already exists as created", func(t *testing.T) {
		t.Parallel()
		identities, fake := newIdentities(t)
		fake.Stub("POST "+identitiesBase+"/identities",
			jsonResponse(http.StatusConflict, `{"message":"exists"}`, "AlreadyExistsException")).
			Stub("GET "+identitiesBase+"/identities/acme.test", jsonResponse(http.StatusOK, identityBody(), ""))

		got, err := identities.Create(t.Context(), "acme.test")

		require.NoError(t, err)
		assert.Len(t, got.Tokens, 3)
	})

	t.Run("reports a rate limit when creating", func(t *testing.T) {
		t.Parallel()
		identities, fake := newIdentities(t)
		fake.Stub("POST "+identitiesBase+"/identities",
			jsonResponse(http.StatusTooManyRequests, `{}`, "TooManyRequestsException"))

		_, err := identities.Create(t.Context(), "acme.test")

		var limited *mail.RateLimitedError
		require.ErrorAs(t, err, &limited)
	})

	t.Run("names an identity it cannot find", func(t *testing.T) {
		t.Parallel()
		identities, fake := newIdentities(t)
		fake.Stub("GET "+identitiesBase+"/identities/gone.test",
			jsonResponse(http.StatusNotFound, `{}`, "NotFoundException"))

		_, err := identities.Get(t.Context(), "gone.test")

		require.ErrorIs(t, err, ses.ErrIdentityNotFound)
		require.ErrorIs(t, err, mail.ErrPermanent)
	})

	t.Run("keeps sending when the return path record is missing", func(t *testing.T) {
		t.Parallel()
		identities, fake := newIdentities(t)
		fake.Stub("PUT "+identitiesBase+"/identities/acme.test/mail-from", jsonResponse(http.StatusOK, `{}`, ""))

		require.NoError(t, identities.SetMailFrom(t.Context(), "acme.test", "bounce.acme.test"))
		fake.AssertSent(t, func(req httpclienttest.SentRequest) bool {
			return strings.Contains(string(req.Body), "USE_DEFAULT_VALUE")
		})
	})

	t.Run("restarts looking for signing records", func(t *testing.T) {
		t.Parallel()
		identities, fake := newIdentities(t)
		fake.Stub("PUT "+identitiesBase+"/identities/acme.test/dkim/signing", jsonResponse(http.StatusOK, `{}`, ""))

		require.NoError(t, identities.RestartDKIM(t.Context(), "acme.test"))
	})

	t.Run("deleting an identity already gone succeeds", func(t *testing.T) {
		t.Parallel()
		identities, fake := newIdentities(t)
		fake.Stub("DELETE "+identitiesBase+"/identities/acme.test",
			jsonResponse(http.StatusNotFound, `{}`, "NotFoundException"))

		require.NoError(t, identities.Delete(t.Context(), "acme.test"))
	})

	t.Run("reports a failed delete", func(t *testing.T) {
		t.Parallel()
		identities, fake := newIdentities(t)
		fake.Stub("DELETE "+identitiesBase+"/identities/acme.test",
			jsonResponse(http.StatusInternalServerError, `{}`, "InternalFailure"))

		require.ErrorIs(t, identities.Delete(t.Context(), "acme.test"), mail.ErrTemporary)
	})

	t.Run("associates a tenant that may already exist", func(t *testing.T) {
		t.Parallel()
		identities, fake := newIdentities(t)
		fake.Stub("POST https://email.eu-west-1.amazonaws.com/v2/email/tenants",
			jsonResponse(http.StatusConflict, `{}`, "AlreadyExistsException")).
			Stub("POST https://email.eu-west-1.amazonaws.com/v2/email/tenants/resources",
				jsonResponse(http.StatusOK, `{}`, ""))

		require.NoError(t, identities.AssociateTenant(t.Context(), "t1", "arn:aws:ses:eu-west-1:x:identity/acme.test"))
	})

	t.Run("lifts a suppressed address, even one not on the list", func(t *testing.T) {
		t.Parallel()
		identities, fake := newIdentities(t)
		fake.Stub("DELETE "+identitiesBase+"/suppression/addresses/a%40b.test",
			jsonResponse(http.StatusNotFound, `{}`, "NotFoundException"))

		require.NoError(t, identities.RemoveSuppressed(t.Context(), "a@b.test"))
	})
}
