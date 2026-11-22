/*
|--------------------------------------------------------------------------
| Operator Routine Scope And Run Requests
|--------------------------------------------------------------------------
|
| What a routine is pointed at, the run a request names, and a run to
| start now.
|
*/

package requests

/*
OperatorRoutineScopeInput is what a routine is pointed at beyond its
inboxes: the help centers it drafts in and the GitHub repositories it
reads.
*/
type OperatorRoutineScopeInput struct {
	HelpCenterIDs []string                         `json:"help_center_ids,omitempty" validate:"omitempty,max=20"`
	GitHub        *OperatorRoutineGitHubScopeInput `json:"github,omitempty"`
}

/*
OperatorRoutineGitHubScopeInput is the GitHub connection a routine reads,
the repositories it may read, the branch merged into, and the labels it
skips.
*/
type OperatorRoutineGitHubScopeInput struct {
	InstallationID string   `json:"installation_id"         validate:"required,max=64"`
	Repositories   []string `json:"repositories"            validate:"required,min=1,max=20"`
	BaseBranch     string   `json:"base_branch,omitempty"   validate:"omitempty,max=200"`
	IgnoreLabels   []string `json:"ignore_labels,omitempty" validate:"omitempty,max=50"`
}

/*
OperatorRoutineRunRequest names one run of a routine.
*/
type OperatorRoutineRunRequest struct {
	Routine string `path:"routine"`
	Run     string `path:"run"`
}

/*
StoreOperatorRoutineRunRequest is the input of POST
/operator/routines/{routine}/runs: a run now, a dry run when asked.
*/
type StoreOperatorRoutineRunRequest struct {
	Routine string `path:"routine"`
	Body    struct {
		DryRun bool `json:"dry_run,omitempty"`
	}
}
