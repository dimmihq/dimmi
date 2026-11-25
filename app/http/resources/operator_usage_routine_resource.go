/*
|--------------------------------------------------------------------------
| Operator Usage By Routine
|--------------------------------------------------------------------------
|
| What each routine's runs cost in the month.
|
*/

package resources

/*
OperatorUsageRoutine is the runs of one routine.
*/
type OperatorUsageRoutine struct {
	RoutineID  string `json:"routine_id"`
	Name       string `json:"name"`
	RunsCount  int64  `json:"runs_count"`
	CostMicros int64  `json:"cost_micros"`
}
