/*
|--------------------------------------------------------------------------
| Widget Incident Controller
|--------------------------------------------------------------------------
|
| The incidents the messenger's Home shows on its status card: those
| customers may see that are active now, each with the customer label
| of its state and its public update log. Anyone on the page may read
| them, with a session or without.
|
*/

package widget

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/actions/tickets"
	actions "github.com/dimmihq/dimmi/app/actions/widget"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
)

/*
IncidentController serves the active incidents.
*/
type IncidentController struct {
	Tickets *tickets.Tickets
}

/*
IncidentsOutput lists the active incidents.
*/
type IncidentsOutput struct {
	Body fhttp.Resource[[]resources.WidgetIncident]
}

/*
Index lists the customer-visible incidents active now, newest first.
*/
func (c *IncidentController) Index(ctx context.Context, _ *requests.NoInput) (*IncidentsOutput, error) {
	resolved, _ := actions.CurrentResolved(ctx)

	live, err := c.Tickets.LiveIncidents(ctx, resolved.Workspace.ID)
	if err != nil {
		return nil, err
	}

	out := &IncidentsOutput{Body: fhttp.NewResource([]resources.WidgetIncident{})}
	for _, inc := range live {
		out.Body.Data = append(out.Body.Data, resources.NewWidgetIncident(inc))
	}

	return out, nil
}
