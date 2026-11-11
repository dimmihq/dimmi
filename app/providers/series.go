/*
|--------------------------------------------------------------------------
| Series
|--------------------------------------------------------------------------
|
| Series' wiring: the engine on top of broadcasts and audiences, the
| jobs that move journeys, and the changes that move them: a contact or
| company changing, an event tracked, and anything that happens to a
| series message.
|
*/

package providers

import (
	"errors"

	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/series"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/jobs"
)

/*
bootSeries builds series on top of broadcasts and registers their jobs.
*/
func bootSeries(a *app.App) error {
	if a.Actions.Broadcasts == nil {
		return nil
	}

	s := &series.Series{
		DB: a.DB, Clock: a.Clock, Queue: a.Queue, Events: a.Events, Audiences: a.Actions.Audiences,
		Broadcasts: a.Actions.Broadcasts, IDs: a.IDs, Limiter: a.Limiter, HTTP: a.HTTP, Config: a.Config.Series,
	}

	a.Actions.Series = s
	a.Actions.Broadcasts.Series = s

	listenForSeriesChanges(a)

	return errors.Join(
		queue.Register(a.Queue, &series.AdvanceWorker{Series: s}),
		queue.Register(a.Queue, &series.CheckWorker{Series: s}),
		queue.Register(a.Queue, &series.EventWorker{Series: s}),
		queue.Register(a.Queue, &series.EnrollWorker{Series: s}),
		queue.Register(a.Queue, &series.CallWorker{Series: s}),
		queue.Register(a.Queue, &series.SweepWorker{Series: s}),
	)
}

/*
listenForSeriesChanges checks people against live series whenever they
change or do something.
*/
func listenForSeriesChanges(a *app.App) {
	events.ListenQueued(a.Events, func(e appevents.ContactCreated) queue.Args {
		return jobs.CheckSeriesSubjectArgs{WorkspaceID: e.WorkspaceID, ContactID: e.ContactID}
	})
	events.ListenQueued(a.Events, func(e appevents.ContactUpdated) queue.Args {
		return jobs.CheckSeriesSubjectArgs{WorkspaceID: e.WorkspaceID, ContactID: e.ContactID}
	})
	events.ListenQueued(a.Events, func(e appevents.CompanyUpdated) queue.Args {
		return jobs.CheckSeriesSubjectArgs{WorkspaceID: e.WorkspaceID, ContactID: ids.Nil(), CompanyID: &e.CompanyID}
	})
	events.ListenQueued(a.Events, func(e appevents.ContactEventTracked) queue.Args {
		return jobs.SeriesEventArgs{WorkspaceID: e.WorkspaceID, ContactID: e.ContactID, Event: e.Name, Ref: e.Ref}
	})
	events.ListenQueued(a.Events, func(e appevents.ContactEventTracked) queue.Args {
		return jobs.CheckSeriesSubjectArgs{WorkspaceID: e.WorkspaceID, ContactID: e.ContactID}
	})
}
