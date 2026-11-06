/*
|--------------------------------------------------------------------------
| AI Schedule
|--------------------------------------------------------------------------
|
| The agent can answer always, only while the team is away, or only
| while the team is around to watch it. Office hours themselves live
| on the agent's settings.
|
*/

package enums

/*
AISchedule names when the agent may answer customers.
*/
type AISchedule string

/*
The schedules.
*/
const (
	AIScheduleAlways        AISchedule = "always"
	AIScheduleOutsideOffice AISchedule = "outside_office_hours"
	AIScheduleInsideOffice  AISchedule = "inside_office_hours"
)
