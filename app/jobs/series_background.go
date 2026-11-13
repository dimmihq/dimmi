/*
|--------------------------------------------------------------------------
| Background Series Jobs
|--------------------------------------------------------------------------
|
| Jobs that run apart from any one change: looking through a series'
| audience, making the calls steps wait on, and the periodic sweep.
|
*/

package jobs

import (
	"time"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"
)

/*
EnrollSeriesArgs looks through the workspace's contacts for those who
match a live series' entry rules. Baseline records the people who match
when the series goes live, so they do not enter; LiveAt is when it went
live, and people changed after it are left to enter by their change.
*/
type EnrollSeriesArgs struct {
	WorkspaceID ids.UUID   `json:"workspace_id"`
	SeriesID    ids.UUID   `json:"series_id"`
	Baseline    bool       `json:"baseline"`
	LiveAt      *time.Time `json:"live_at,omitempty"`
}

/*
Kind names the job.
*/
func (EnrollSeriesArgs) Kind() string { return "enroll_series" }

/*
Options keep one look per series waiting at a time.
*/
func (EnrollSeriesArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: 5, Unique: queue.UniqueOpts{ByArgs: true, ByState: pendingStates()}}
}

/*
CallSeriesStepArgs makes the call one step waits on, an AI path or a
webhook, outside the transaction that moves the journey.
*/
type CallSeriesStepArgs struct {
	WorkspaceID  ids.UUID `json:"workspace_id"`
	EnrollmentID ids.UUID `json:"enrollment_id"`
	Seq          int64    `json:"seq"`
}

/*
Kind names the job.
*/
func (CallSeriesStepArgs) Kind() string { return "call_series_step" }

/*
Options try a call three times; the step's own timeout takes the
fallback path after that.
*/
func (CallSeriesStepArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: 3, Unique: queue.UniqueOpts{ByArgs: true, ByState: pendingStates()}}
}

/*
SweepSeriesArgs names the periodic sweep that looks for new matches,
late goals and lost jobs; it takes no arguments.
*/
type SweepSeriesArgs struct{}

/*
Kind names the job.
*/
func (SweepSeriesArgs) Kind() string { return "sweep_series" }

/*
Options run the sweep once every few minutes at most.
*/
func (SweepSeriesArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: 3, Unique: queue.UniqueOpts{ByPeriod: 5 * time.Minute}}
}
