/*
|--------------------------------------------------------------------------
| Portal Status
|--------------------------------------------------------------------------
|
| The help center's status page, public to everyone and linked from the
| header of every page: one line for the whole product, each affected
| area with its current status, the incidents active now with their
| public update log, and those resolved in the last ninety days. The
| sample portal shows every system operational.
|
*/

package web

import (
	"time"

	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/actions/tickets"
	"github.com/dimmihq/dimmi/resources/views/helpcenter"
)

/*
Status shows the help center's status page.
*/
func (c PortalController) Status(w fhttp.ResponseWriter, r *fhttp.Request) {
	s, live, ok := c.live(w, r)
	if !ok {
		return
	}

	if !live || c.Requests == nil {
		center, found := c.center(w, r)
		if found {
			c.respond(w, r, fhttp.StatusOK, helpcenter.StatusPage(center, helpcenter.SystemStatus{}, c.styles()))
		}

		return
	}

	board, err := c.Requests.Board(r.Context(), s.help.WorkspaceID)
	if err != nil {
		c.Errors.Render(w, r, err)

		return
	}

	w.Header().Set("Cache-Control", "public, max-age=30")
	c.respond(w, r, fhttp.StatusOK, helpcenter.StatusPage(s.center, boardView(board), c.styles()))
}

/*
boardView shapes the status board for its page.
*/
func boardView(b tickets.StatusBoard) helpcenter.SystemStatus {
	out := helpcenter.SystemStatus{Overall: b.Overall}

	for _, comp := range b.Components {
		out.Components = append(out.Components, helpcenter.Component{Name: comp.Name, Status: comp.Status})
	}

	for _, inc := range b.Active {
		out.Active = append(out.Active, incidentView(inc))
	}

	for _, inc := range b.History {
		out.History = append(out.History, incidentView(inc))
	}

	return out
}

/*
incidentView shapes an incident for the status page: resolved ones read
as operational, active ones by their severity.
*/
func incidentView(inc tickets.StatusIncident) helpcenter.Incident {
	status := tickets.SeverityStatus(tickets.Severity(inc.Ticket))
	if inc.Ticket.StateGroup == tickets.GroupResolved {
		status = tickets.StatusOperational
	}

	out := helpcenter.Incident{
		Title: inc.Ticket.Title, Label: inc.State.CustomerLabel, Status: status,
		Areas: tickets.Areas(inc.Ticket), Started: stamp(inc.Ticket.CreatedAt),
	}

	if inc.Ticket.ResolvedAt != nil {
		out.Resolved = stamp(*inc.Ticket.ResolvedAt)
	}

	for _, u := range inc.Updates {
		out.Updates = append(out.Updates, helpcenter.IncidentPost{Body: u.Body, At: stamp(u.CreatedAt)})
	}

	return out
}

/*
stamp writes when something happened, in UTC.
*/
func stamp(t time.Time) string {
	return t.UTC().Format("Jan 2, 15:04 UTC")
}
