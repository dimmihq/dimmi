package provider_test

import (
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/mail/internal/provider"
)

func TestCheck(t *testing.T) {
	t.Parallel()

	statuses := map[string]struct {
		status int
		kind   error
	}{
		"accepts a 2xx answer":                   {http.StatusAccepted, nil},
		"refuses a validation error for good":    {http.StatusUnprocessableEntity, mail.ErrPermanent},
		"tries later when the key is refused":    {http.StatusForbidden, mail.ErrTemporary},
		"tries later when the request timed out": {http.StatusRequestTimeout, mail.ErrTemporary},
		"tries later after a server error":       {http.StatusBadGateway, mail.ErrTemporary},
	}

	for name, answer := range statuses {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := provider.Check("acme", &httpclient.Response{Status: answer.status}, nil)

			if answer.kind == nil {
				require.NoError(t, err)

				return
			}

			require.ErrorIs(t, err, answer.kind)
		})
	}

	t.Run("reads how long a rate limit asks to wait", func(t *testing.T) {
		t.Parallel()

		for header, wait := range map[string]time.Duration{"7": 7 * time.Second, "soon": 0, "-1": 0} {
			resp := &httpclient.Response{
				Status: http.StatusTooManyRequests, Header: http.Header{"Retry-After": {header}},
			}

			var limited *mail.RateLimitedError
			require.ErrorAs(t, provider.Check("acme", resp, nil), &limited)
			assert.Equal(t, wait, limited.RetryAfter, header)
		}
	})

	t.Run("treats a request without an answer as temporary", func(t *testing.T) {
		t.Parallel()
		errNetwork := errors.New("connection reset")

		err := provider.Check("acme", nil, errNetwork)

		require.ErrorIs(t, err, mail.ErrTemporary)
		require.ErrorIs(t, err, errNetwork)
	})

	t.Run("keeps only the start of a long answer", func(t *testing.T) {
		t.Parallel()
		body := strings.Repeat("é", 400)

		err := provider.Check("acme", &httpclient.Response{Status: http.StatusBadRequest, Body: []byte(body)}, nil)

		assert.Less(t, len(err.Error()), 400)
		assert.True(t, strings.HasSuffix(err.Error(), "...: mail: permanent failure"), err.Error())
	})
}

func TestAddresses(t *testing.T) {
	t.Parallel()

	t.Run("lists bare and formatted addresses", func(t *testing.T) {
		t.Parallel()
		list := []mail.Address{{Name: "Ana", Email: "ana@example.com"}, {Email: "bo@example.com"}}

		assert.Equal(t, []string{"ana@example.com", "bo@example.com"}, provider.Emails(list))
		assert.Equal(t, []string{`"Ana" <ana@example.com>`, "<bo@example.com>"}, provider.Formatted(list))
	})
}
