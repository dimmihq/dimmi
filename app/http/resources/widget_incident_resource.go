/*
|--------------------------------------------------------------------------
| Widget Incident Resources
|--------------------------------------------------------------------------
|
| Incidents as customers read them on the messenger's status card and
| the help center's status page: the title, the customer label of the
| state, severity and affected areas, and the public update log, newest
| first. Nothing the team keeps to itself shows.
|
*/

package resources

import (
	"encoding/json"
	"time"

	"github.com/dimmihq/dimmi/app/actions/tickets"
	"github.com/dimmihq/dimmi/app/models"
)

/*
WidgetIncidentUpdate is one post of an incident's public log.
*/
type WidgetIncidentUpdate struct {
	ID        string    `json:"id"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

/*
WidgetIncident is an incident as customers read it. Latest is its
newest public post, nil before the first.
*/
type WidgetIncident struct {
	ID         string                 `json:"id"`
	Object     string                 `json:"object"`
	Title      string                 `json:"title"`
	State      WidgetTicketState      `json:"state"`
	Severity   *string                `json:"severity"`
	Areas      []string               `json:"areas"`
	Latest     *WidgetIncidentUpdate  `json:"latest_update"`
	Updates    []WidgetIncidentUpdate `json:"updates"`
	CreatedAt  time.Time              `json:"created_at"`
	ResolvedAt *time.Time             `json:"resolved_at"`
}

/*
NewWidgetIncident shapes an incident for customers.
*/
func NewWidgetIncident(inc tickets.StatusIncident) WidgetIncident {
	r := inc.Ticket
	values := struct {
		Severity *string  `json:"severity"`
		Areas    []string `json:"areas"`
	}{}
	_ = json.Unmarshal(r.Fields, &values)

	out := WidgetIncident{
		ID: models.PublicID(models.PrefixTicket, r.ID), Object: "incident", Title: r.Title,
		State:    WidgetTicketState{Label: inc.State.CustomerLabel, Group: inc.State.StateGroup},
		Severity: values.Severity, Areas: values.Areas, Updates: []WidgetIncidentUpdate{},
		CreatedAt: r.CreatedAt.UTC(), ResolvedAt: r.ResolvedAt,
	}

	if out.Areas == nil {
		out.Areas = []string{}
	}

	for _, u := range inc.Updates {
		out.Updates = append(out.Updates, WidgetIncidentUpdate{
			ID: models.PublicID(models.PrefixTicketPublicUpdate, u.ID), Body: u.Body, CreatedAt: u.CreatedAt.UTC(),
		})
	}

	if len(out.Updates) > 0 {
		out.Latest = &out.Updates[0]
	}

	return out
}
