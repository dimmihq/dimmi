/*
|--------------------------------------------------------------------------
| TLS Driver Contract
|--------------------------------------------------------------------------
|
| Whichever driver an installation runs, the application must be able to
| rely on the same things: a hostname the authorizer refuses is refused,
| a hostname it allows can be ensured, removing is always safe to repeat,
| and plain HTTP requests still reach the application. A driver that
| orders certificates is given a backend that already holds one for
| Host, so the suite never reaches a certificate authority.
|
|   func TestDriver(t *testing.T) {
|       drivertest.Run(t, newDriver)
|   }
|
*/

package drivertest

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/customdomain"
)

/*
The hostnames the suite uses: Host is authorized, Refused is not.
*/
const (
	Host    = "docs.example.com"
	Refused = "refused.example.com"
)

/*
ErrRefused is the reason Authorize gives for refusing a hostname.
*/
var ErrRefused = errors.New("drivertest: hostname not verified")

/*
NewTLS returns a driver that asks the authorizer about hostnames and,
if it orders certificates, already holds one for Host.
*/
type NewTLS func(t *testing.T, authorize customdomain.Authorizer) customdomain.TLS

/*
Authorize allows Host and refuses every other hostname.
*/
func Authorize(_ context.Context, host string) error {
	if host == Host {
		return nil
	}

	return ErrRefused
}

/*
Run runs the contract against drivers from the constructor.
*/
func Run(t *testing.T, newTLS NewTLS) {
	t.Helper()

	t.Run("ensures an authorized hostname", func(t *testing.T) {
		t.Parallel()

		require.NoError(t, newTLS(t, Authorize).Ensure(t.Context(), Host))
	})

	t.Run("refuses a hostname the authorizer refuses", func(t *testing.T) {
		t.Parallel()

		err := newTLS(t, Authorize).Ensure(t.Context(), Refused)

		require.ErrorIs(t, err, customdomain.ErrNotAuthorized)
		require.ErrorIs(t, err, ErrRefused)
	})

	t.Run("removes hostnames repeatedly whether ensured or not", func(t *testing.T) {
		t.Parallel()
		driver := newTLS(t, Authorize)

		require.NoError(t, driver.Remove(t.Context(), Host))
		require.NoError(t, driver.Remove(t.Context(), Host))
		require.NoError(t, driver.Remove(t.Context(), Refused))
	})

	t.Run("passes plain requests to the application", func(t *testing.T) {
		t.Parallel()
		next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })
		recorder := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "http://"+Host+"/pricing", http.NoBody)

		newTLS(t, Authorize).HTTPHandler(next).ServeHTTP(recorder, request)

		assert.Equal(t, http.StatusTeapot, recorder.Code)
	})

	t.Run("finds certificates during the handshake when it terminates tls", func(t *testing.T) {
		t.Parallel()

		cfg := newTLS(t, Authorize).TLSConfig()
		if cfg != nil {
			assert.NotNil(t, cfg.GetCertificate)
		}
	})
}
