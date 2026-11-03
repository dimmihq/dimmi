/*
|--------------------------------------------------------------------------
| Realtime Connections
|--------------------------------------------------------------------------
|
| The realtime server starts before the providers that build the
| guards, so it is handed this method rather than the resolver itself.
| Until the resolver exists, during boot, every connection is refused.
|
*/

package app

import (
	"context"

	fbroadcasting "github.com/dimmihq/dimmi/framework/broadcasting"
	fhttp "github.com/dimmihq/dimmi/framework/http"
)

/*
ResolveRealtime authenticates a realtime connection with the resolver
the providers built.
*/
func (a *App) ResolveRealtime(ctx context.Context, r *fhttp.Request, token string) (fbroadcasting.Principal, error) {
	if a.Realtime == nil {
		return fbroadcasting.Principal{}, fbroadcasting.ErrUnauthenticated
	}

	return a.Realtime.Resolve(ctx, r, token)
}
