package provider_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/mail/internal/provider"
)

func TestCheckDomain(t *testing.T) {
	t.Parallel()

	statuses := map[string]struct {
		status int
		kinds  []error
	}{
		"tells a missing domain apart": {http.StatusNotFound, []error{mail.ErrDomainNotFound, mail.ErrPermanent}},
		"tells a refused credential apart": {
			http.StatusUnauthorized, []error{mail.ErrDomainAccessDenied, mail.ErrTemporary},
		},
		"tells a forbidden credential apart": {
			http.StatusForbidden, []error{mail.ErrDomainAccessDenied, mail.ErrTemporary},
		},
		"refuses a bad request for good":   {http.StatusUnprocessableEntity, []error{mail.ErrPermanent}},
		"tries later after a server error": {http.StatusBadGateway, []error{mail.ErrTemporary}},
	}

	for name, answer := range statuses {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := provider.CheckDomain("acme", "get domain", &httpclient.Response{Status: answer.status}, nil)

			for _, kind := range answer.kinds {
				require.ErrorIs(t, err, kind)
			}
		})
	}

	t.Run("accepts a 2xx answer", func(t *testing.T) {
		t.Parallel()

		require.NoError(t, provider.CheckDomain("acme", "get domain", &httpclient.Response{Status: http.StatusOK}, nil))
	})

	t.Run("treats a request without an answer as temporary", func(t *testing.T) {
		t.Parallel()
		errNetwork := errors.New("connection reset")

		err := provider.CheckDomain("acme", "get domain", nil, errNetwork)

		require.ErrorIs(t, err, mail.ErrTemporary)
		require.ErrorIs(t, err, errNetwork)
		assert.Contains(t, err.Error(), "acme get domain")
	})
}

func TestTrackingLeftOn(t *testing.T) {
	t.Parallel()

	t.Run("keeps the cause", func(t *testing.T) {
		t.Parallel()

		err := provider.TrackingLeftOn("acme", mail.ErrTemporary)

		require.ErrorIs(t, err, mail.ErrDomainTrackingLeftOn)
		require.ErrorIs(t, err, mail.ErrTemporary)
	})
}

func TestDecodeDomain(t *testing.T) {
	t.Parallel()

	t.Run("reads a good answer", func(t *testing.T) {
		t.Parallel()

		var into struct{ ID string }
		resp := &httpclient.Response{Body: []byte(`{"id":"7"}`)}

		require.NoError(t, provider.DecodeDomain("acme", "get domain", resp, &into))
		assert.Equal(t, "7", into.ID)
	})

	t.Run("treats a garbled answer as temporary", func(t *testing.T) {
		t.Parallel()

		var into struct{ ID string }
		err := provider.DecodeDomain("acme", "get domain", &httpclient.Response{Body: []byte(`oops`)}, &into)

		require.ErrorIs(t, err, mail.ErrTemporary)
	})
}

func TestFullHost(t *testing.T) {
	t.Parallel()

	hosts := map[string]string{
		"":                  "example.com",
		"@":                 "example.com",
		"example.com":       "example.com",
		"send":              "send.example.com",
		"send.example.com":  "send.example.com",
		"send.example.com.": "send.example.com",
		"resend._domainkey": "resend._domainkey.example.com",
		"notexample.com":    "notexample.com.example.com",
	}

	t.Run("qualifies a host under the domain once", func(t *testing.T) {
		t.Parallel()

		for host, want := range hosts {
			assert.Equal(t, want, provider.FullHost(host, "example.com"), host)
		}
	})
}
