/*
|--------------------------------------------------------------------------
| Engineering Tool Controller
|--------------------------------------------------------------------------
|
| POST /webhooks/linear, /webhooks/jira and /webhooks/github-issues are
| where the apps dimmi registered deliver; /webhooks/incident-io,
| /webhooks/rootly and /webhooks/pagerduty end in the installation whose
| endpoint the workspace created, since each signs with its own secret.
| A body larger than any delivery is refused before anything reads it;
| a bad signature is 401 and an address naming nothing 404. Everything
| else is answered 200 once applied, so a tool retries a failure.
|
*/

package webhooks

import (
	"errors"
	"io"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/log"

	"github.com/dimmihq/dimmi/app/actions/issues"
	"github.com/dimmihq/dimmi/app/integrations/trackers"
	"github.com/dimmihq/dimmi/app/models"
)

/*
maxDelivery bounds a tool's delivery.
*/
const maxDelivery = 2 << 20

/*
TrackerController receives engineering tools' deliveries.
*/
type TrackerController struct {
	Issues *issues.Issues
	IDs    *ids.Registry
}

/*
Linear receives the Linear app's deliveries.
*/
func (c *TrackerController) Linear(w fhttp.ResponseWriter, r *fhttp.Request) {
	c.serve(w, r, "linear", "")
}

/*
Jira receives the Jira app's deliveries.
*/
func (c *TrackerController) Jira(w fhttp.ResponseWriter, r *fhttp.Request) {
	c.serve(w, r, "jira", "")
}

/*
GitHub receives a repository webhook's deliveries; the address names
the installation that added it.
*/
func (c *TrackerController) GitHub(w fhttp.ResponseWriter, r *fhttp.Request) {
	c.serve(w, r, "github", r.URL.Query().Get("installation"))
}

/*
IncidentIO receives an incident.io endpoint's deliveries.
*/
func (c *TrackerController) IncidentIO(w fhttp.ResponseWriter, r *fhttp.Request) {
	c.serve(w, r, "incident_io", r.PathValue("installation"))
}

/*
Rootly receives a Rootly endpoint's deliveries.
*/
func (c *TrackerController) Rootly(w fhttp.ResponseWriter, r *fhttp.Request) {
	c.serve(w, r, "rootly", r.PathValue("installation"))
}

/*
PagerDuty receives a PagerDuty subscription's deliveries.
*/
func (c *TrackerController) PagerDuty(w fhttp.ResponseWriter, r *fhttp.Request) {
	c.serve(w, r, "pagerduty", r.PathValue("installation"))
}

/*
serve reads the delivery within the bound and hands it to the action.
*/
func (c *TrackerController) serve(w fhttp.ResponseWriter, r *fhttp.Request, provider, installation string) {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxDelivery+1))
	if err != nil {
		w.WriteHeader(fhttp.StatusBadRequest)

		return
	}

	if len(body) > maxDelivery {
		w.WriteHeader(fhttp.StatusRequestEntityTooLarge)

		return
	}

	d := issues.Delivery{Provider: provider, Signed: trackers.Signed{Header: r.Header, Body: body}}

	if installation != "" {
		id, err := c.IDs.Decode(string(models.PrefixIntegrationInstallation), installation)
		if err != nil {
			w.WriteHeader(fhttp.StatusNotFound)

			return
		}

		d.InstallationID = &id
	}

	switch err := c.Issues.Receive(r.Context(), d); {
	case err == nil:
		w.WriteHeader(fhttp.StatusOK)
	case errors.Is(err, issues.ErrUnsigned):
		w.WriteHeader(fhttp.StatusUnauthorized)
	case errors.Is(err, issues.ErrUnknownTool):
		w.WriteHeader(fhttp.StatusNotFound)
	default:
		log.From(r.Context()).WarnContext(r.Context(), "tool delivery failed", log.String("provider", provider),
			log.Err(err))
		w.WriteHeader(fhttp.StatusInternalServerError)
	}
}
