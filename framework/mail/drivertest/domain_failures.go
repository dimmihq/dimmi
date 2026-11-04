/*
|--------------------------------------------------------------------------
| Domain Manager Failures
|--------------------------------------------------------------------------
|
| A caller decides on retries and on what to show from the kind of error
| alone, so every call of every domain manager must report each way a
| provider can fail as the same kind.
|
*/

package drivertest

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/mail"
)

/*
operation is one DomainManager call and the routes it reaches first.
Decodes says it reads an answer's body, and takesID that it names an
existing domain.
*/
type operation struct {
	call    func(ctx context.Context, manager mail.DomainManager) error
	routes  []DomainRoute
	decodes bool
	takesID bool
}

func (c domainContract) operations() map[string]operation {
	id := c.api.ID

	return map[string]operation{
		"create": {
			call: func(ctx context.Context, m mail.DomainManager) error {
				_, err := m.CreateDomain(ctx, "example.com")

				return err
			},
			routes: []DomainRoute{c.api.Create}, decodes: true,
		},
		"get": {
			call: func(ctx context.Context, m mail.DomainManager) error {
				_, err := m.GetDomain(ctx, id)

				return err
			},
			routes: []DomainRoute{c.api.Read}, decodes: true, takesID: true,
		},
		"verify": {
			call: func(ctx context.Context, m mail.DomainManager) error {
				_, err := m.VerifyDomain(ctx, id)

				return err
			},
			routes: c.api.Verify, decodes: true, takesID: true,
		},
		"delete": {
			call:   func(ctx context.Context, m mail.DomainManager) error { return m.DeleteDomain(ctx, id) },
			routes: []DomainRoute{c.api.Delete}, takesID: true,
		},
	}
}

func (c domainContract) runFailures(t *testing.T) {
	t.Helper()

	for name, op := range c.operations() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c.runStatuses(t, op)
			c.runRateLimit(t, op)
			c.runNoAnswer(t, op)

			if op.decodes {
				c.runGarbled(t, op)
			}

			if op.takesID {
				c.runNotFound(t, op)
			}
		})
	}

	t.Run("treats a blank id as not found without a request", func(t *testing.T) {
		t.Parallel()
		manager, _ := c.start(t)

		_, getErr := manager.GetDomain(t.Context(), "")
		_, verifyErr := manager.VerifyDomain(t.Context(), "")

		require.ErrorIs(t, getErr, mail.ErrDomainNotFound)
		require.ErrorIs(t, verifyErr, mail.ErrDomainNotFound)
		require.ErrorIs(t, manager.DeleteDomain(t.Context(), ""), mail.ErrDomainNotFound)
	})
}

func (c domainContract) runStatuses(t *testing.T, op operation) {
	t.Helper()

	statuses := map[string]struct {
		status int
		kinds  []error
	}{
		"refused credential":   {http.StatusUnauthorized, []error{mail.ErrDomainAccessDenied, mail.ErrTemporary}},
		"forbidden credential": {http.StatusForbidden, []error{mail.ErrDomainAccessDenied, mail.ErrTemporary}},
		"server error":         {http.StatusInternalServerError, []error{mail.ErrTemporary}},
		"refused request":      {http.StatusUnprocessableEntity, []error{mail.ErrPermanent}},
	}

	for name, status := range statuses {
		t.Run("maps a "+name, func(t *testing.T) {
			t.Parallel()
			refusal := httpclient.Response{Status: status.status, Body: []byte(`{"message":"refused"}`)}
			manager, _ := c.start(t, answering(op.routes, refusal)...)

			err := op.call(t.Context(), manager)

			for _, kind := range status.kinds {
				require.ErrorIs(t, err, kind)
			}
		})
	}
}

func (c domainContract) runRateLimit(t *testing.T, op operation) {
	t.Helper()

	t.Run("maps a rate limit with its wait", func(t *testing.T) {
		t.Parallel()
		limited := httpclient.Response{
			Status: http.StatusTooManyRequests, Header: http.Header{"Retry-After": {"7"}},
		}
		manager, _ := c.start(t, answering(op.routes, limited)...)

		var rate *mail.RateLimitedError
		require.ErrorAs(t, op.call(t.Context(), manager), &rate)
		assert.Equal(t, 7*time.Second, rate.RetryAfter)
	})
}

func (c domainContract) runNoAnswer(t *testing.T, op operation) {
	t.Helper()

	t.Run("treats a request without an answer as temporary", func(t *testing.T) {
		t.Parallel()
		manager, _ := c.start(t)

		require.ErrorIs(t, op.call(t.Context(), manager), mail.ErrTemporary)
	})

	t.Run("stops when the context is canceled", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		manager, _ := c.start(t, op.routes...)

		err := op.call(ctx, manager)

		require.ErrorIs(t, err, mail.ErrTemporary)
		require.ErrorIs(t, err, context.Canceled)
	})
}

func (c domainContract) runGarbled(t *testing.T, op operation) {
	t.Helper()

	t.Run("treats a garbled answer as temporary", func(t *testing.T) {
		t.Parallel()
		manager, _ := c.start(t, answering(op.routes, httpclient.Response{Body: []byte("not json")})...)

		require.ErrorIs(t, op.call(t.Context(), manager), mail.ErrTemporary)
	})
}

func (c domainContract) runNotFound(t *testing.T, op operation) {
	t.Helper()

	t.Run("maps a missing domain", func(t *testing.T) {
		t.Parallel()
		manager, _ := c.start(t, answering(op.routes, c.api.NotFound)...)

		err := op.call(t.Context(), manager)

		require.ErrorIs(t, err, mail.ErrDomainNotFound)
		require.ErrorIs(t, err, mail.ErrPermanent)
	})
}

/*
answering returns the routes with every answer replaced.
*/
func answering(routes []DomainRoute, answer httpclient.Response) []DomainRoute {
	out := make([]DomainRoute, 0, len(routes))
	for _, route := range routes {
		out = append(out, DomainRoute{Pattern: route.Pattern, Answer: answer})
	}

	return out
}

/*
failingLast returns a copy of the routes whose last one answers with a
server error.
*/
func failingLast(routes []DomainRoute) []DomainRoute {
	out := append([]DomainRoute{}, routes...)
	out[len(out)-1].Answer = httpclient.Response{Status: http.StatusBadGateway}

	return out
}
