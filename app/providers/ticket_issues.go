/*
|--------------------------------------------------------------------------
| Linked Issue Wiring
|--------------------------------------------------------------------------
|
| Tickets link to issues and incidents in engineering tools: the
| trackers for each tool, the action that links, receives and re-checks
| them, the job that re-checks links every fifteen minutes and the one
| that writes the impact back when a ticket's links change.
|
*/

package providers

import (
	"context"
	"errors"

	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/issues"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/integrations/trackers"
	"github.com/dimmihq/dimmi/app/jobs"
)

/*
bootTicketIssues builds the linked issues action and its jobs.
*/
func bootTicketIssues(a *app.App) error {
	if a.Actions.Tickets == nil || a.Actions.Integrations == nil {
		return nil
	}

	s := &issues.Issues{
		DB: a.DB, Clock: a.Clock, Queue: a.Queue, Tickets: a.Actions.Tickets,
		Tokens:   a.Actions.Integrations.Tokens,
		Registry: a.Actions.Integrations.Registry, Services: a.Config.Services, AppURL: a.Application.Config.App.URL,
		Trackers: trackers.Set{
			"linear": trackers.NewLinear(a.HTTP), "jira": trackers.NewJira(a.HTTP),
			"github":      trackers.NewGitHub(a.HTTP),
			"incident_io": trackers.NewIncidentIO(a.HTTP), "rootly": trackers.NewRootly(a.HTTP),
			"pagerduty": trackers.NewPagerDuty(a.HTTP),
		},
	}
	fakeTrackers(a, s)
	a.Actions.TicketIssues = s

	refresh := func(ctx context.Context, e appevents.TicketChanged) error {
		return a.Queue.Push(context.WithoutCancel(ctx), jobs.RefreshTicketImpactArgs{
			WorkspaceID: e.WorkspaceID, TicketID: e.TicketID,
		})
	}
	events.Listen(a.Events, events.AfterCommit, func(ctx context.Context, e appevents.TicketLinked) error {
		return refresh(ctx, e.TicketChanged)
	})
	events.Listen(a.Events, events.AfterCommit, func(ctx context.Context, e appevents.TicketUnlinked) error {
		return refresh(ctx, e.TicketChanged)
	})

	return errors.Join(
		queue.Register(a.Queue, &jobs.SyncTicketIssues{Issues: s}),
		queue.Register(a.Queue, &jobs.RefreshTicketImpact{Issues: s}),
	)
}
