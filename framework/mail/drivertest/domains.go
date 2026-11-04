/*
|--------------------------------------------------------------------------
| Domain Manager Contract
|--------------------------------------------------------------------------
|
| Every domain manager must report a provider's domains the same way, or
| switching provider would change what a caller shows and when it retries.
| A driver's tests call RunDomains with the provider's fixture and a
| constructor. Requests reach only a fake transport, and any request the
| fixture does not list fails the test.
|
|   func TestDomains(t *testing.T) {
|       drivertest.RunDomains(t, drivertest.PostmarkDomains(), newManager)
|   }
|
*/

package drivertest

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
	"github.com/dimmihq/dimmi/framework/mail"
)

/*
DomainRoute is one provider endpoint, as "METHOD exact-url", and what it
answers.
*/
type DomainRoute struct {
	Pattern string
	Answer  httpclient.Response
}

/*
DomainAPI describes a provider's domain API for the contract.

ID is the id of the domain in the answers, and Records its records before
they pass. Create and Read answer with the domain before it passes. Verify
lists every request a verification makes, answering as if DNS is now
right. Tracking lists the requests that turn tracking off after creating.
NotFound is the provider's answer for a domain it does not have.
Credential says whether a request carries DomainToken.
*/
type DomainAPI struct {
	ID         string
	Records    []mail.DomainRecord
	Create     DomainRoute
	Read       DomainRoute
	Delete     DomainRoute
	Verify     []DomainRoute
	Tracking   []DomainRoute
	NotFound   httpclient.Response
	Credential func(req httpclienttest.SentRequest) bool
}

/*
NewDomainManager returns the manager under test on the client, using
DomainToken as its credential.
*/
type NewDomainManager func(client *httpclient.Client) mail.DomainManager

/*
RunDomains runs the contract against a provider's fixture.
*/
func RunDomains(t *testing.T, api DomainAPI, newManager NewDomainManager) {
	t.Helper()

	c := domainContract{api: api, newManager: newManager}

	t.Run("create", func(t *testing.T) { t.Parallel(); c.runCreate(t) })
	t.Run("read", func(t *testing.T) { t.Parallel(); c.runRead(t) })
	t.Run("verify", func(t *testing.T) { t.Parallel(); c.runVerify(t) })
	t.Run("delete", func(t *testing.T) { t.Parallel(); c.runDelete(t) })
	t.Run("failures", func(t *testing.T) { t.Parallel(); c.runFailures(t) })
}

type domainContract struct {
	api        DomainAPI
	newManager NewDomainManager
}

/*
start returns a manager whose transport answers only the routes.
*/
func (c domainContract) start(t *testing.T, routes ...DomainRoute) (mail.DomainManager, *httpclienttest.Fake) {
	t.Helper()

	client, err := httpclient.New(httpclient.Config{AllowFakes: true})
	require.NoError(t, err)

	fake := httpclienttest.New(client).PreventStrayRequests()
	for _, route := range routes {
		fake.Stub(route.Pattern, route.Answer)
	}

	return c.newManager(client), fake
}

/*
assertCalled fails unless each route was requested with the credential.
*/
func (c domainContract) assertCalled(t *testing.T, fake *httpclienttest.Fake, routes ...DomainRoute) {
	t.Helper()

	for _, route := range routes {
		fake.AssertSent(t, func(req httpclienttest.SentRequest) bool {
			return req.Method+" "+req.URL.String() == route.Pattern && c.api.Credential(req)
		})
	}
}

func (c domainContract) runCreate(t *testing.T) {
	t.Helper()

	t.Run("returns the records to publish and turns tracking off", func(t *testing.T) {
		t.Parallel()
		routes := append([]DomainRoute{c.api.Create}, c.api.Tracking...)
		manager, fake := c.start(t, routes...)

		domain, err := manager.CreateDomain(t.Context(), "example.com")

		require.NoError(t, err)
		assert.Equal(t, mail.ProviderDomain{ID: c.api.ID, Records: c.api.Records}, domain)
		c.assertCalled(t, fake, routes...)
	})

	if len(c.api.Tracking) > 0 {
		c.runTrackingFailures(t)
	}
}

func (c domainContract) runTrackingFailures(t *testing.T) {
	t.Helper()

	t.Run("keeps the domain when tracking cannot be turned off", func(t *testing.T) {
		t.Parallel()
		refused := httpclient.Response{Status: http.StatusForbidden}
		manager, _ := c.start(t, append(answering(c.api.Tracking, refused), c.api.Create)...)

		domain, err := manager.CreateDomain(t.Context(), "example.com")

		require.ErrorIs(t, err, mail.ErrDomainTrackingLeftOn)
		require.ErrorIs(t, err, mail.ErrDomainAccessDenied)
		assert.Equal(t, c.api.ID, domain.ID)
		assert.Equal(t, c.api.Records, domain.Records)
	})

	t.Run("reports tracking left on when a later switch fails", func(t *testing.T) {
		t.Parallel()
		routes := failingLast(c.api.Tracking)
		manager, _ := c.start(t, append(routes, c.api.Create)...)

		_, err := manager.CreateDomain(t.Context(), "example.com")

		require.ErrorIs(t, err, mail.ErrDomainTrackingLeftOn)
		require.ErrorIs(t, err, mail.ErrTemporary)
	})
}

func (c domainContract) runRead(t *testing.T) {
	t.Helper()

	t.Run("reads the domain", func(t *testing.T) {
		t.Parallel()
		manager, fake := c.start(t, c.api.Read)

		domain, err := manager.GetDomain(t.Context(), c.api.ID)

		require.NoError(t, err)
		assert.Equal(t, mail.ProviderDomain{ID: c.api.ID, Records: c.api.Records}, domain)
		c.assertCalled(t, fake, c.api.Read)
	})
}

func (c domainContract) runVerify(t *testing.T) {
	t.Helper()

	t.Run("flips verified and every record once dns is right", func(t *testing.T) {
		t.Parallel()
		manager, fake := c.start(t, c.api.Verify...)

		domain, err := manager.VerifyDomain(t.Context(), c.api.ID)

		require.NoError(t, err)
		assert.Equal(t, c.api.ID, domain.ID)
		assert.True(t, domain.Verified)
		require.NotEmpty(t, domain.Records)

		for _, record := range domain.Records {
			assert.True(t, record.Valid, record.Host)
		}

		c.assertCalled(t, fake, c.api.Verify...)
	})

	t.Run("fails when the last step of a verification fails", func(t *testing.T) {
		t.Parallel()
		manager, _ := c.start(t, failingLast(c.api.Verify)...)

		_, err := manager.VerifyDomain(t.Context(), c.api.ID)

		require.ErrorIs(t, err, mail.ErrTemporary)
	})
}

func (c domainContract) runDelete(t *testing.T) {
	t.Helper()

	t.Run("removes the domain", func(t *testing.T) {
		t.Parallel()
		manager, fake := c.start(t, c.api.Delete)

		require.NoError(t, manager.DeleteDomain(t.Context(), c.api.ID))
		c.assertCalled(t, fake, c.api.Delete)
	})
}
