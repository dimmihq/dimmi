/*
|--------------------------------------------------------------------------
| Operator Agent Requests
|--------------------------------------------------------------------------
|
| What an agent doing Operator's work asks beside its suggestions: the
| suggestions made lately, the routine whose state it reads or saves or
| reports for, the skill it reads and the knowledge it searches.
|
*/

package requests

/*
RecentOperatorSuggestionsRequest filters GET /operator/suggestions/recent.
*/
type RecentOperatorSuggestionsRequest struct {
	DaysBack  int    `query:"days_back"  minimum:"1" maximum:"90"`
	Status    string `query:"status"     enum:"pending,applied,declined,reverted,failed"`
	RoutineID string `query:"routine_id" maxLength:"64"`
}

/*
OperatorRoutineStateRequest names the routine whose state is read.
*/
type OperatorRoutineStateRequest struct {
	Routine string `path:"routine"`
}

/*
SaveOperatorRoutineStateRequest is the input of PUT
/operator/routines/{routine}/state.
*/
type SaveOperatorRoutineStateRequest struct {
	Routine string `path:"routine"`
	Body    struct {
		State             map[string]any `json:"state"                        validate:"required"`
		ReplaceEverything bool           `json:"replace_everything,omitempty"`
		DryRun            bool           `json:"dry_run,omitempty"`
	}
}

/*
StoreNothingToChangeRequest is the input of POST
/operator/routines/{routine}/nothing-to-change.
*/
type StoreNothingToChangeRequest struct {
	Routine string `path:"routine"`
	Body    struct {
		Reason string `json:"reason"            validate:"required,max=500"`
		DryRun bool   `json:"dry_run,omitempty"`
	}
}
