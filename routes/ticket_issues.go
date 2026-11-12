/*
|--------------------------------------------------------------------------
| Ticket Issue Routes
|--------------------------------------------------------------------------
|
| The engineering work behind tickets: issues linked to a ticket, the
| lists and search of each connected tool its pickers read, a tool's
| sync settings, connecting a tool that takes an API key, the drafts
| closing the loop, and the workspace's ticket switches. Reading takes
| tickets:read and changing tickets:write; a tool's settings and the
| workspace's switches take settings:write. The tools deliver to
| signed web routes outside the API.
|
*/

package routes

import (
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/routing"

	"github.com/dimmihq/dimmi/app"
	v1 "github.com/dimmihq/dimmi/app/http/controllers/api/v1"
	"github.com/dimmihq/dimmi/app/http/controllers/webhooks"
)

/*
ticketIssueRoutes declares linked issues and the tools behind them.
*/
func ticketIssueRoutes(workspace *routing.Router, a *app.App) {
	if a.Actions.TicketIssues == nil {
		return
	}

	ic := &v1.TicketIssueController{
		TicketController: v1.TicketController{Tickets: a.Actions.Tickets, IDs: a.IDs}, Issues: a.Actions.TicketIssues,
	}
	one := "/tickets/{ticket}"

	routing.Get(workspace, one+"/issues", "tickets.issues.index", ic.Index).Ability("tickets:read")
	routing.Post(workspace, one+"/issues", "tickets.issues.store", ic.Store).Ability("tickets:write").
		Status(fhttp.StatusCreated)
	routing.Delete(workspace, "/ticket-issues/{issue}", "ticket-issues.destroy", ic.Destroy).
		Ability("tickets:write").Status(fhttp.StatusNoContent)
	routing.Get(workspace, one+"/update-drafts", "tickets.update-drafts.index", ic.Drafts).Ability("tickets:read")
	routing.Patch(workspace, "/ticket-update-drafts/{draft}", "ticket-update-drafts.update", ic.UpdateDraft).
		Ability("tickets:write")

	issueToolRoutes(workspace, a)
}

/*
issueToolRoutes declares the tools' lists, search and settings, and
the workspace's ticket switches.
*/
func issueToolRoutes(workspace *routing.Router, a *app.App) {
	tc := &v1.IssueToolController{
		IDs: a.IDs, Issues: a.Actions.TicketIssues, Integrations: a.Actions.Integrations.CreateManual,
		List: a.Actions.Integrations.List,
	}
	inst := "/integration-installations/{installation}"

	routing.Get(workspace, inst+"/issue-teams", "integration-installations.issue-teams.index", tc.Teams).
		Ability("tickets:read")
	routing.Get(workspace, inst+"/issue-options", "integration-installations.issue-options.index", tc.Options).
		Ability("tickets:read")
	routing.Get(workspace, inst+"/issues", "integration-installations.issues.index", tc.Search).
		Ability("tickets:read")
	routing.Get(workspace, inst+"/issue-settings", "integration-installations.issue-settings.show", tc.Settings).
		Ability("settings:read")
	routing.Patch(workspace, inst+"/issue-settings", "integration-installations.issue-settings.update",
		tc.UpdateSettings).Ability("settings:write")
	routing.Post(workspace, "/integrations/{provider}/key-installations", "integrations.key-installations.store",
		tc.StoreKey).Ability("settings:write").Status(fhttp.StatusCreated)
	routing.Get(workspace, "/ticket-settings", "ticket-settings.show", tc.TicketSettings).Ability("tickets:read")
	routing.Patch(workspace, "/ticket-settings", "ticket-settings.update", tc.UpdateTicketSettings).
		Ability("settings:write")
}

/*
trackerWebhookRoutes declares where engineering tools deliver.
*/
func trackerWebhookRoutes(a *app.App) {
	if a.Actions.TicketIssues == nil {
		return
	}

	c := &webhooks.TrackerController{Issues: a.Actions.TicketIssues, IDs: a.IDs}

	for _, r := range []struct {
		path, name string
		handler    fhttp.HandlerFunc
	}{
		{"/webhooks/linear", "webhooks.linear", c.Linear},
		{"/webhooks/jira", "webhooks.jira", c.Jira},
		{"/webhooks/github-issues", "webhooks.github-issues", c.GitHub},
		{"/webhooks/incident-io/{installation}", "webhooks.incident-io", c.IncidentIO},
		{"/webhooks/rootly/{installation}", "webhooks.rootly", c.Rootly},
		{"/webhooks/pagerduty/{installation}", "webhooks.pagerduty", c.PagerDuty},
	} {
		a.Router.Handle(fhttp.MethodPost, r.path, r.name, r.handler)
	}
}
