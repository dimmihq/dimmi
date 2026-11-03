/*
|--------------------------------------------------------------------------
| Console Routes
|--------------------------------------------------------------------------
|
| Work the scheduler runs on its own. Every hour the sign-in records that
| ran out are purged: unfinished authenticator enrollments, spent mailed
| links and codes, expired password resets and sessions nobody can
| resume.
|
*/

package routes

import (
	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/jobs"
)

/*
Console declares the scheduled jobs.
*/
func Console(a *app.App) {
	a.Schedule.Job(jobs.PurgeExpiredAuthRecordsArgs{}).Hourly()
}
