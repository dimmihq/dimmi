/*
|--------------------------------------------------------------------------
| Series Jobs
|--------------------------------------------------------------------------
|
| A journey moves in small jobs. Advancing takes one enrollment as far as
| it can go now; it is pushed when someone enters, when a wait ends and
| whenever something the journey may wait on changes. A change to a
| person is checked once for every live series, so they can enter, leave
| or move on. Calls that leave the database, an AI path or a webhook,
| run in their own job and hand the result back. The periodic jobs find
| people who began to match as time passed and push any job that was
| lost.
|
*/

package jobs

import (
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"
)

/*
AdvanceSeriesEnrollmentArgs moves one enrollment on. Seq is the
sequence the job was pushed for, so a timer that outlived its wait does
nothing; zero means move on from wherever the enrollment stands.
*/
type AdvanceSeriesEnrollmentArgs struct {
	WorkspaceID  ids.UUID `json:"workspace_id"`
	EnrollmentID ids.UUID `json:"enrollment_id"`
	Seq          int64    `json:"seq"`
}

/*
Kind names the job.
*/
func (AdvanceSeriesEnrollmentArgs) Kind() string { return "advance_series_enrollment" }

/*
Options are not unique: a job pushed while another runs must still run
after it, and a job with nothing to do costs one locked read.
*/
func (AdvanceSeriesEnrollmentArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: 10}
}

/*
CheckSeriesSubjectArgs checks one contact against every live series of
the workspace: entering, leaving, goals and waits. With only CompanyID,
it moves the journeys of the company and its people after the company
changed.
*/
type CheckSeriesSubjectArgs struct {
	WorkspaceID ids.UUID  `json:"workspace_id"`
	ContactID   ids.UUID  `json:"contact_id"`
	CompanyID   *ids.UUID `json:"company_id,omitempty"`
}

/*
Kind names the job.
*/
func (CheckSeriesSubjectArgs) Kind() string { return "check_series_subject" }

/*
Options are not unique, so a change that lands while a check runs is
checked too.
*/
func (CheckSeriesSubjectArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: 5}
}

/*
SeriesEventArgs offers an event a contact did to the series it starts.
Ref names the event, so the same event never enrolls anyone twice.
*/
type SeriesEventArgs struct {
	WorkspaceID ids.UUID `json:"workspace_id"`
	ContactID   ids.UUID `json:"contact_id"`
	Event       string   `json:"event"`
	Ref         string   `json:"ref"`
}

/*
Kind names the job.
*/
func (SeriesEventArgs) Kind() string { return "series_event" }

/*
Options retry the offer a few times.
*/
func (SeriesEventArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: 5}
}
