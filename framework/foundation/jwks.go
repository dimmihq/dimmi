/*
|--------------------------------------------------------------------------
| Published Keys
|--------------------------------------------------------------------------
|
| Tokens from identity providers are verified against the keys the
| provider publishes. Those keys are fetched through the guarded HTTP
| client like every other outbound call, so a key URL can never reach a
| private address, and fakes answer it in tests.
|
*/

package foundation

import (
	"context"
	"fmt"
	"net/http"

	"github.com/dimmihq/dimmi/framework/jwt"
)

/*
JWKS returns a cache of the keys published at the URL, fetched through
the application's HTTP client and timed by its clock.
*/
func (a *Application) JWKS(url string, opts jwt.JWKSOptions) *jwt.JWKS {
	if opts.Clock == nil {
		opts.Clock = a.Clock
	}

	return jwt.NewJWKS(url, a.fetch, opts)
}

/*
fetch returns the body at the URL, refusing any answer but 200.
*/
func (a *Application) fetch(ctx context.Context, url string) ([]byte, error) {
	resp, err := a.HTTP.New().Accept("application/json").Get(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", url, err)
	}

	if resp.Status != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: status %d", url, resp.Status)
	}

	return resp.Body, nil
}
