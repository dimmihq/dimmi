/*
|--------------------------------------------------------------------------
| Client Mutation Id
|--------------------------------------------------------------------------
|
| A screen that changes something shows the change at once and then
| hears about it again from the realtime server. It names each change in
| the Dimmi-Client-Mutation-Id header; events carry the name back in
| their broadcast, so the screen recognizes its own echo and drops it
| instead of applying the change twice.
|
*/

package middleware

import (
	"strings"

	fhttp "github.com/dimmihq/dimmi/framework/http"
)

/*
ClientMutationIDHeader names the header, and MaxClientMutationID caps
the length of its value.
*/
const (
	ClientMutationIDHeader = "Dimmi-Client-Mutation-Id"
	MaxClientMutationID    = 64
)

/*
ClientMutationID returns the id the client gave the change the request
makes, cut to MaxClientMutationID bytes, or "" when it gave none.
*/
func ClientMutationID(r *fhttp.Request) string {
	id := strings.TrimSpace(r.Header.Get(ClientMutationIDHeader))
	if len(id) > MaxClientMutationID {
		return id[:MaxClientMutationID]
	}

	return id
}
