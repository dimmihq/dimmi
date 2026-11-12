/*
|--------------------------------------------------------------------------
| Report Wiring
|--------------------------------------------------------------------------
|
| Reports are a registry of sources: the conversation reports register
| here, and other areas, tickets among them, register their own once
| they boot. The facts behind them are kept fresh by the refresh job,
| the backlog is counted nightly and digests go out each morning.
|
*/

package providers

import (
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/reports"
	"github.com/dimmihq/dimmi/app/jobs"
	appmail "github.com/dimmihq/dimmi/app/mail"
)

/*
bootReports builds the reports, their upkeep and their digests.
*/
func bootReports(a *app.App) {
	r := &reports.Reports{DB: a.DB, Clock: a.Clock}
	for _, s := range []reports.Source{
		reports.Overview{},
		reports.Responsiveness{},
		reports.Resolution{},
		reports.Team{},
		reports.Live{},
		reports.Satisfaction{},
		reports.AI{},
		reports.HelpCenter{},
	} {
		r.Register(s)
	}

	a.Actions.Reports = r
	a.Actions.ReportMaintenance = &reports.Maintenance{
		Builder: &reports.Builder{DB: a.DB, Clock: a.Clock}, IDs: a.IDs,
	}
	a.Actions.ReportDigests = &reports.Digests{Reports: r, Mail: a.Mail, AppURL: a.Links.Base}
	appmail.RegisterReportDigests(a.Mail)
}

/*
reportWork is what the report jobs run: upkeep and digests.
*/
type reportWork struct {
	*reports.Maintenance
	*reports.Digests
}

/*
registerReportJobs registers the report jobs' workers.
*/
func registerReportJobs(a *app.App) error {
	work := reportWork{a.Actions.ReportMaintenance, a.Actions.ReportDigests}

	if err := queue.Register(a.Queue, &jobs.RefreshReports{Work: work, Now: a.Clock.Now}); err != nil {
		return err
	}

	if err := queue.Register(a.Queue, &jobs.SnapshotBacklogs{Work: work}); err != nil {
		return err
	}

	return queue.Register(a.Queue, &jobs.SendReportDigests{Work: work})
}
