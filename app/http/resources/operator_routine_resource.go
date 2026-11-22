/*
|--------------------------------------------------------------------------
| Operator Routine Resources
|--------------------------------------------------------------------------
|
| How routines read over the API: what a routine runs and when, what it
| may read and change, where it reports, how its last run went and what
| it spent this month; each run with its outcome, cost and thread; and
| the templates a routine starts from.
|
*/

package resources

import (
	"encoding/json"
	"time"

	"github.com/dimmihq/dimmi/app/actions/operatorchat"
	"github.com/dimmihq/dimmi/app/ai/operator"
	"github.com/dimmihq/dimmi/app/models"
)

/*
OperatorRoutineGitHubScope is the GitHub connection and repositories a
routine reads.
*/
type OperatorRoutineGitHubScope struct {
	InstallationID string   `json:"installation_id"`
	Repositories   []string `json:"repositories"`
	BaseBranch     *string  `json:"base_branch"`
	IgnoreLabels   []string `json:"ignore_labels"`
}

/*
OperatorRoutineScope is what a routine is pointed at beyond its inboxes.
*/
type OperatorRoutineScope struct {
	HelpCenterIDs []string                    `json:"help_center_ids"`
	GitHub        *OperatorRoutineGitHubScope `json:"github"`
}

/*
OperatorRoutineDelivery is where a routine reports: teammates told in
the app, Slack destinations, and teammates emailed.
*/
type OperatorRoutineDelivery struct {
	InAppMemberIDs      []string `json:"in_app_member_ids"`
	SlackDestinationIDs []string `json:"slack_destination_ids"`
	EmailMemberIDs      []string `json:"email_member_ids"`
}

/*
OperatorRoutine is one routine. NextRuns are its next three runs;
MonthSpendMicros is what its runs cost this month.
*/
type OperatorRoutine struct {
	ID                     string                  `json:"id"`
	Object                 string                  `json:"object"`
	Name                   string                  `json:"name"`
	Description            string                  `json:"description"`
	TemplateKey            *string                 `json:"template_key"`
	Prompt                 string                  `json:"prompt"`
	Cadence                string                  `json:"cadence"`
	CronExpression         *string                 `json:"cron_expression"`
	RunAt                  *time.Time              `json:"run_at"`
	Timezone               string                  `json:"timezone"`
	NextRunAt              *time.Time              `json:"next_run_at"`
	NextRuns               []time.Time             `json:"next_runs"`
	CreatedBy              *string                 `json:"created_by"`
	OwnerID                *string                 `json:"owner_id"`
	Permissions            []string                `json:"permissions"`
	ToolGroups             []string                `json:"tool_groups"`
	InboxIDs               []string                `json:"inbox_ids"`
	WritePolicies          map[string]string       `json:"write_policies"`
	Scope                  OperatorRoutineScope    `json:"scope"`
	Model                  *string                 `json:"model"`
	Effort                 *string                 `json:"effort"`
	MaxModelCalls          int32                   `json:"max_model_calls"`
	MaxToolCalls           int32                   `json:"max_tool_calls"`
	MonthlyBudgetMicros    *int64                  `json:"monthly_budget_micros"`
	MonthSpendMicros       int64                   `json:"month_spend_micros"`
	NotifyPolicy           string                  `json:"notify_policy"`
	Delivery               OperatorRoutineDelivery `json:"delivery"`
	State                  json.RawMessage         `json:"state"`
	Status                 string                  `json:"status"`
	PauseReason            *string                 `json:"pause_reason"`
	ConsecutiveFailures    int32                   `json:"consecutive_failures"`
	MaxConsecutiveFailures int32                   `json:"max_consecutive_failures"`
	LastRunAt              *time.Time              `json:"last_run_at"`
	LastSuccessAt          *time.Time              `json:"last_success_at"`
	LastOutcome            *string                 `json:"last_outcome"`
	CreatedAt              time.Time               `json:"created_at"`
	UpdatedAt              time.Time               `json:"updated_at"`
}

/*
NewOperatorRoutine shapes a routine with its next runs and spend.
*/
func NewOperatorRoutine(v operatorchat.RoutineView) OperatorRoutine {
	r := v.Routine
	out := OperatorRoutine{
		ID: models.PublicID(models.PrefixOperatorRoutine, r.ID), Object: "operator_routine", Name: r.Name,
		Description: r.Description, TemplateKey: r.TemplateKey, Prompt: r.Prompt, Cadence: r.Cadence,
		CronExpression: r.CronExpr, RunAt: r.RunAt, Timezone: r.Timezone, NextRunAt: r.NextRunAt,
		NextRuns:  orTimes(v.NextRuns),
		CreatedBy: optionalID(models.PrefixMember, r.CreatedByMemberID),
		OwnerID:   optionalID(models.PrefixMember, r.OwnerMemberID), Permissions: orStrings(r.Permissions),
		ToolGroups: orStrings(r.ToolGroups), InboxIDs: publicIDs(models.PrefixInbox, r.InboxIds),
		WritePolicies: map[string]string{}, Scope: routineScope(r.Scope), Model: r.Model, Effort: r.Effort,
		MaxModelCalls: r.MaxModelCalls, MaxToolCalls: r.MaxToolCalls, MonthlyBudgetMicros: r.MonthlyBudgetMicros,
		MonthSpendMicros: v.MonthSpend, NotifyPolicy: r.NotifyPolicy, Delivery: routineDelivery(r.Delivery),
		State: r.State, Status: r.Status, PauseReason: r.PauseReason, ConsecutiveFailures: r.ConsecutiveFailures,
		MaxConsecutiveFailures: r.MaxConsecutiveFailures, LastRunAt: r.LastRunAt, LastSuccessAt: r.LastSuccessAt,
		LastOutcome: r.LastOutcome, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
	_ = json.Unmarshal(r.WritePolicies, &out.WritePolicies)
	if len(out.State) == 0 {
		out.State = json.RawMessage("{}")
	}
	return out
}

/*
NewOperatorRoutines shapes routines as a list shows them.
*/
func NewOperatorRoutines(rows []models.OperatorRoutine) []OperatorRoutine {
	out := make([]OperatorRoutine, len(rows))
	for i, r := range rows {
		out[i] = NewOperatorRoutine(operatorchat.RoutineView{Routine: r})
	}
	return out
}

/*
routineScope shapes a routine's scope.
*/
func routineScope(raw json.RawMessage) OperatorRoutineScope {
	s := operator.ScopeOf(raw)
	out := OperatorRoutineScope{HelpCenterIDs: publicIDs(models.PrefixHelpCenter, s.HelpCenterIDs)}
	if s.GitHub != nil {
		out.GitHub = &OperatorRoutineGitHubScope{
			InstallationID: models.PublicID(models.PrefixIntegrationInstallation, s.GitHub.InstallationID),
			Repositories:   orStrings(s.GitHub.Repositories), BaseBranch: optionalText(s.GitHub.BaseBranch),
			IgnoreLabels: orStrings(s.GitHub.IgnoreLabels),
		}
	}
	return out
}

/*
routineDelivery shapes where a routine reports, as stored.
*/
func routineDelivery(raw json.RawMessage) OperatorRoutineDelivery {
	var out OperatorRoutineDelivery
	_ = json.Unmarshal(raw, &out)
	out.InAppMemberIDs, out.SlackDestinationIDs = orStrings(out.InAppMemberIDs), orStrings(out.SlackDestinationIDs)
	out.EmailMemberIDs = orStrings(out.EmailMemberIDs)
	return out
}

/*
orStrings is a list, empty when there is none.
*/
func orStrings(list []string) []string {
	if list == nil {
		return []string{}
	}
	return list
}

/*
orTimes is a list of moments, empty when there is none.
*/
func orTimes(list []time.Time) []time.Time {
	if list == nil {
		return []time.Time{}
	}
	return list
}
