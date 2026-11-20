/*
|--------------------------------------------------------------------------
| Messenger And Instagram Webhooks
|--------------------------------------------------------------------------
|
| Meta posts a Page's and an Instagram account's events here, one address
| for each platform. A check of the address echoes Meta's challenge when
| its token is the install's. A delivery is read within a size bound and
| handed to the platform's channel, which answers before any of it is
| processed, as Meta wants within seconds.
|
*/

package webhooks

import (
	"io"

	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/actions/channels"
	"github.com/dimmihq/dimmi/app/drivers/channel/meta"
)

/*
SocialController receives Meta's deliveries for one platform. MaxBytes
bounds a body.
*/
type SocialController struct {
	Channel  *channels.Social
	MaxBytes int64
}

/*
Check answers Meta's check of the address.
*/
func (c *SocialController) Check(w fhttp.ResponseWriter, r *fhttp.Request) {
	query := r.URL.Query()

	challenge, ok := c.Channel.Verify(channels.SocialCheck{
		Mode: query.Get("hub.mode"), Token: query.Get("hub.verify_token"), Challenge: query.Get("hub.challenge"),
	})
	if !ok {
		w.WriteHeader(fhttp.StatusForbidden)

		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(fhttp.StatusOK)
	_, _ = io.WriteString(w, challenge)
}

/*
Events receives the platform's deliveries.
*/
func (c *SocialController) Events(w fhttp.ResponseWriter, r *fhttp.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, c.MaxBytes+1))
	if err != nil {
		w.WriteHeader(fhttp.StatusBadRequest)

		return
	}

	if int64(len(body)) > c.MaxBytes {
		w.WriteHeader(fhttp.StatusRequestEntityTooLarge)

		return
	}

	outcome, err := c.Channel.Receive(r.Context(), channels.SocialDelivery{
		Signature: r.Header.Get(meta.SignatureHeader), Body: body,
	})
	if err != nil {
		w.WriteHeader(fhttp.StatusInternalServerError)

		return
	}

	w.WriteHeader(statusOf(outcome))
}
