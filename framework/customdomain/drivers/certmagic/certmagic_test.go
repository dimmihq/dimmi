package certmagic_test

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"

	magic "github.com/caddyserver/certmagic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/customdomain"
	"github.com/dimmihq/dimmi/framework/customdomain/drivers/certmagic"
	"github.com/dimmihq/dimmi/framework/customdomain/drivertest"
	"github.com/dimmihq/dimmi/framework/customdomain/storage"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/database/migrate"
	"github.com/dimmihq/dimmi/framework/testkit"
)

/*
openDB returns a pool on a fresh database migrated with the storage
table.
*/
func openDB(t *testing.T) *database.DB {
	t.Helper()

	url := testkit.NewDatabase(t).URL
	_, err := migrate.Runner{URL: url, Framework: storage.Migrations()}.Up(t.Context())
	require.NoError(t, err)

	db, err := database.Open(t.Context(), database.Config{URL: url, MaxConns: 6, PoolMode: database.PoolModeSession})
	require.NoError(t, err)
	t.Cleanup(db.Close)

	return db
}

/*
brokenCA serves an ACME directory URL that answers every request with
a server error, so an order fails without reaching a real authority.
*/
func brokenCA(t *testing.T) string {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	return server.URL + "/directory"
}

/*
newDriver returns a driver on a fresh database whose authority always
fails, holding a certificate for every hostname given.
*/
func newDriver(t *testing.T, authorize customdomain.Authorizer, stored ...string) *certmagic.Driver {
	t.Helper()

	return newLoggedDriver(t, authorize, nil, stored...)
}

/*
newLoggedDriver is newDriver with the library's log lines sent to the
logger.
*/
func newLoggedDriver(
	t *testing.T, authorize customdomain.Authorizer, logger *zap.Logger, stored ...string,
) *certmagic.Driver {
	t.Helper()

	db := openDB(t)
	ca := brokenCA(t)
	driver, err := certmagic.Factory(db, authorize, logger)("edge", config.Section{
		"driver": "certmagic",
		"email":  "ops@example.com",
		"ca":     ca,
	})
	require.NoError(t, err)

	typed, ok := driver.(*certmagic.Driver)
	require.True(t, ok)
	t.Cleanup(typed.Stop)

	issuer := (&magic.ACMEIssuer{CA: ca}).IssuerKey()
	store := storage.New(db, clock.System())

	for _, host := range stored {
		for _, key := range []string{
			magic.StorageKeys.SiteCert(issuer, host),
			magic.StorageKeys.SitePrivateKey(issuer, host),
			magic.StorageKeys.SiteMeta(issuer, host),
		} {
			require.NoError(t, store.Store(t.Context(), key, []byte("stored")))
		}
	}

	return typed
}

func TestDriver(t *testing.T) {
	t.Parallel()

	drivertest.Run(t, func(t *testing.T, authorize customdomain.Authorizer) customdomain.TLS {
		t.Helper()

		return newDriver(t, authorize, drivertest.Host)
	})

	t.Run("reports an order the authority fails", func(t *testing.T) {
		t.Parallel()
		driver := newDriver(t, func(context.Context, string) error { return nil })

		err := driver.Ensure(t.Context(), "new.example.com")

		require.ErrorContains(t, err, "obtain certificate for new.example.com")
	})

	t.Run("sends the library's log lines to the logger", func(t *testing.T) {
		t.Parallel()
		core, logs := observer.New(zap.DebugLevel)
		driver := newLoggedDriver(t, func(context.Context, string) error { return nil }, zap.New(core))

		require.Error(t, driver.Ensure(t.Context(), "logged.example.com"))

		assert.NotZero(t, logs.Len())
	})

	t.Run("deletes the stored certificate on remove", func(t *testing.T) {
		t.Parallel()
		driver := newDriver(t, drivertest.Authorize, drivertest.Host)

		require.NoError(t, driver.Remove(t.Context(), drivertest.Host))

		require.Error(t, driver.Ensure(t.Context(), drivertest.Host))
	})

	t.Run("refuses a handshake for a hostname the authorizer refuses", func(t *testing.T) {
		t.Parallel()
		cfg := newDriver(t, drivertest.Authorize).TLSConfig()

		_, err := cfg.GetCertificate(&tls.ClientHelloInfo{ServerName: drivertest.Refused})

		require.ErrorContains(t, err, "not authorized")
		assert.Equal(t, []string{"acme-tls/1", "h2", "http/1.1"}, cfg.NextProtos)
	})

	t.Run("rejects options it cannot decode", func(t *testing.T) {
		t.Parallel()

		_, err := certmagic.Factory(nil, drivertest.Authorize, nil)("edge", config.Section{"email": []int{1}})

		require.Error(t, err)
	})

	t.Run("requires an email so it never prompts", func(t *testing.T) {
		t.Parallel()

		_, err := certmagic.Factory(nil, drivertest.Authorize, nil)("edge", config.Section{"driver": "certmagic"})

		require.ErrorIs(t, err, certmagic.ErrEmailRequired)
	})
}
