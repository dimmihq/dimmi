/*
|--------------------------------------------------------------------------
| WhatsApp Controller
|--------------------------------------------------------------------------
|
| /webhooks/whatsapp is where the install's own Meta app delivers; the
| same path ending in a connection's id is where a workspace's own app
| delivers. A GET is Meta checking the address, answered with its
| challenge when the token is right. A POST is a delivery: a body larger
| than Meta ever sends is refused before anything reads it, and the rest
| is the channel's, the signature checked on the raw body.
|
*/

package webhooks

import (
	"io"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/channels"
	wadriver "github.com/dimmihq/dimmi/app/drivers/channel/whatsapp"
	"github.com/dimmihq/dimmi/app/models"
)

/*
WhatsAppController receives Meta's deliveries. MaxBytes bounds a body.
*/
type WhatsAppController struct {
	WhatsApp *channels.WhatsApp
	IDs      *ids.Registry
	MaxBytes int64
}

/*
Check answers Meta's check of the shared address.
*/
func (c *WhatsAppController) Check(w fhttp.ResponseWriter, r *fhttp.Request) {
	c.answerCheck(w, r, nil)
}

/*
ConnectionCheck answers Meta's check of a connection's own address.
*/
func (c *WhatsAppController) ConnectionCheck(w fhttp.ResponseWriter, r *fhttp.Request) {
	id, ok := c.connection(w, r)
	if ok {
		c.answerCheck(w, r, &id)
	}
}

/*
Events receives the shared app's deliveries.
*/
func (c *WhatsAppController) Events(w fhttp.ResponseWriter, r *fhttp.Request) {
	c.serve(w, r, nil)
}

/*
ConnectionEvents receives the deliveries of a workspace's own app.
*/
func (c *WhatsAppController) ConnectionEvents(w fhttp.ResponseWriter, r *fhttp.Request) {
	id, ok := c.connection(w, r)
	if ok {
		c.serve(w, r, &id)
	}
}

/*
connection reads the connection the path names; an id of anything else
leads nowhere.
*/
func (c *WhatsAppController) connection(w fhttp.ResponseWriter, r *fhttp.Request) (ids.UUID, bool) {
	id, err := c.IDs.Decode(string(models.PrefixChannelConnection), r.PathValue("channel"))
	if err != nil {
		w.WriteHeader(fhttp.StatusNotFound)

		return ids.Nil(), false
	}

	return id, true
}

/*
answerCheck echoes the challenge when the token is the one the address
expects.
*/
func (c *WhatsAppController) answerCheck(w fhttp.ResponseWriter, r *fhttp.Request, connectionID *ids.UUID) {
	query := r.URL.Query()

	challenge, ok, err := c.WhatsApp.Verify(r.Context(), channels.WhatsAppCheck{
		ConnectionID: connectionID, Mode: query.Get("hub.mode"), Token: query.Get("hub.verify_token"),
		Challenge: query.Get("hub.challenge"),
	})
	if err != nil {
		w.WriteHeader(fhttp.StatusInternalServerError)

		return
	}

	if !ok {
		w.WriteHeader(fhttp.StatusForbidden)

		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(fhttp.StatusOK)
	_, _ = io.WriteString(w, challenge)
}

/*
serve reads the delivery within the size bound and hands it to the
channel.
*/
func (c *WhatsAppController) serve(w fhttp.ResponseWriter, r *fhttp.Request, connectionID *ids.UUID) {
	body, err := io.ReadAll(io.LimitReader(r.Body, c.MaxBytes+1))
	if err != nil {
		w.WriteHeader(fhttp.StatusBadRequest)

		return
	}

	if int64(len(body)) > c.MaxBytes {
		w.WriteHeader(fhttp.StatusRequestEntityTooLarge)

		return
	}

	outcome, err := c.WhatsApp.Receive(r.Context(), channels.WhatsAppDelivery{
		ConnectionID: connectionID, Signature: r.Header.Get(wadriver.SignatureHeader), Body: body,
	})
	if err != nil {
		w.WriteHeader(fhttp.StatusInternalServerError)

		return
	}

	w.WriteHeader(statusOf(outcome))
}
