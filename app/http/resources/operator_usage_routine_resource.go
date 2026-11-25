/*
|--------------------------------------------------------------------------
| Operator Usage By Routine
|--------------------------------------------------------------------------
|
| How often each routine ran in the month.
|
*/

package resources

/*
OperatorUsageRoutine is the runs of one routine.
*/
type OperatorUsageRoutine struct {
	RoutineID string `json:"routine_id"`
	Name      string `json:"name"`
	RunsCount int64  `json:"runs_count"`
}
