/*
|--------------------------------------------------------------------------
| Slack Controller
|--------------------------------------------------------------------------
|
| POST /webhooks/slack/events and /webhooks/slack/interactions are where
| the install's shared Slack app delivers; the same paths ending in a
| connection's id are where a workspace's own app delivers. A body larger
| than any Slack delivery is refused before anything reads it. The rest
| is the channel's: the signature checked on the raw body, the delivery
| kept and queued, and Slack answered well within its three seconds.
|
| An interaction arrives as a form with one payload field; the signature
| covers the whole form as sent.
|
*/

package webhooks

import (
	"encoding/json"
	"io"
	"net/url"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/channels"
	"github.com/dimmihq/dimmi/app/models"
)

/*
SlackController receives Slack's deliveries. MaxBytes bounds a body.
*/
type SlackController struct {
	Slack    *channels.Slack
	IDs      *ids.Registry
	MaxBytes int64
}

/*
Events receives the shared app's events.
*/
func (c *SlackController) Events(w fhttp.ResponseWriter, r *fhttp.Request) {
	c.serve(w, r, slackAddress{})
}

/*
Interactions receives the shared app's interactions.
*/
func (c *SlackController) Interactions(w fhttp.ResponseWriter, r *fhttp.Request) {
	c.serve(w, r, slackAddress{interaction: true})
}

/*
ConnectionEvents receives the events of a workspace's own app.
*/
func (c *SlackController) ConnectionEvents(w fhttp.ResponseWriter, r *fhttp.Request) {
	c.serveConnection(w, r, slackAddress{})
}

/*
ConnectionInteractions receives the interactions of a workspace's own
app.
*/
func (c *SlackController) ConnectionInteractions(w fhttp.ResponseWriter, r *fhttp.Request) {
	c.serveConnection(w, r, slackAddress{interaction: true})
}

/*
serveConnection receives a delivery to the connection the path names;
an id of anything else leads nowhere.
*/
func (c *SlackController) serveConnection(w fhttp.ResponseWriter, r *fhttp.Request, to slackAddress) {
	id, err := c.IDs.Decode(string(models.PrefixChannelConnection), r.PathValue("channel"))
	if err != nil {
		w.WriteHeader(fhttp.StatusNotFound)

		return
	}

	to.connectionID = &id
	c.serve(w, r, to)
}

/*
slackAddress is what a delivery was sent to: a connection's own address
or the shared one, for events or interactions.
*/
type slackAddress struct {
	connectionID *ids.UUID
	interaction  bool
}

/*
serve reads the delivery within the size bound, hands it to the channel
and answers Slack.
*/
func (c *SlackController) serve(w fhttp.ResponseWriter, r *fhttp.Request, to slackAddress) {
	body, err := io.ReadAll(io.LimitReader(r.Body, c.MaxBytes+1))
	if err != nil {
		w.WriteHeader(fhttp.StatusBadRequest)

		return
	}

	if int64(len(body)) > c.MaxBytes {
		w.WriteHeader(fhttp.StatusRequestEntityTooLarge)

		return
	}

	d := channels.Delivery{
		ConnectionID: to.connectionID, Interaction: to.interaction, Body: body,
		Timestamp: r.Header.Get("X-Slack-Request-Timestamp"), Signature: r.Header.Get("X-Slack-Signature"),
	}

	if to.interaction {
		form, err := url.ParseQuery(string(body))
		if err != nil {
			w.WriteHeader(fhttp.StatusBadRequest)

			return
		}

		d.Payload = []byte(form.Get("payload"))
	}

	answer, err := c.Slack.Receive(r.Context(), d)
	if err != nil {
		w.WriteHeader(fhttp.StatusInternalServerError)

		return
	}

	respond(w, answer)
}

/*
respond writes the answer, echoing an address check's challenge.
*/
func respond(w fhttp.ResponseWriter, answer channels.Answer) {
	status := statusOf(answer.Outcome)
	if answer.Challenge == "" {
		w.WriteHeader(status)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"challenge": answer.Challenge})
}

/*
statusOf is the status Slack reads each outcome by: success stops its
retries, a refused signature is unauthorized.
*/
func statusOf(outcome channels.Outcome) int {
	switch outcome {
	case channels.Accepted:
		return fhttp.StatusOK
	case channels.Unsigned:
		return fhttp.StatusUnauthorized
	case channels.Unknown:
		return fhttp.StatusNotFound
	case channels.Unreadable:
		return fhttp.StatusBadRequest
	default:
		return fhttp.StatusInternalServerError
	}
}
