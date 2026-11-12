/*
|--------------------------------------------------------------------------
| Series Events
|--------------------------------------------------------------------------
|
| A series going live with a new version, and a person's journey through
| it ending. Each is dispatched in the transaction that made the change.
| Test journeys never dispatch.
|
*/

package events

import "github.com/dimmihq/dimmi/framework/ids"

/*
SeriesPublished says a series' working copy became its live version.
*/
type SeriesPublished struct {
	Base
	SeriesID  ids.UUID
	VersionID ids.UUID
}

/*
EventName names the event.
*/
func (SeriesPublished) EventName() string { return "series.published" }

/*
SeriesJourneyCompleted says a journey ended: the traveler finished a
path, reached the goal or left. Reason is the journey's end reason and
Goal whether they reached the goal on the way.
*/
type SeriesJourneyCompleted struct {
	Base
	SeriesID     ids.UUID
	EnrollmentID ids.UUID
	Reason       string
	Goal         bool
}

/*
EventName names the event.
*/
func (SeriesJourneyCompleted) EventName() string { return "series.journey_completed" }
