/*
|--------------------------------------------------------------------------
| Routine Records
|--------------------------------------------------------------------------
|
| How a routine is written: whole on create and update, and what each
| change did in the audit log.
|
*/

package operatorchat

import (
	"context"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/app/actions/audit"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
)

/*
routineEvent is what happened to a routine and what it changed.
*/
type routineEvent struct {
	name    string
	changes map[string]any
}

/*
record writes an audit entry about a routine.
*/
func (a *Routines) record(ctx context.Context, m authz.Membership, r models.OperatorRoutine, e routineEvent) error {
	event, changes := e.name, e.changes
	if a.Audit == nil {
		return nil
	}
	_, err := a.Audit.Handle(ctx, audit.Entry{
		WorkspaceID: m.Workspace.ID, Event: event, ActorID: m.Member.ID,
		Subject: audit.Subject{Type: "operator_routine", ID: r.ID, Label: r.Name}, Changes: changes,
	})
	return err
}

/*
changed lists the settings an update changed.
*/
func changed(before, after models.OperatorRoutine) map[string]any {
	out := map[string]any{}
	pairs := map[string][2]any{
		"name": {before.Name, after.Name}, "prompt": {before.Prompt, after.Prompt},
		"cadence": {before.Cadence, after.Cadence}, "status": {before.Status, after.Status},
		"cron_expression": {deref(before.CronExpr), deref(after.CronExpr)},
		"timezone":        {before.Timezone, after.Timezone},
		"permissions":     {strings.Join(before.Permissions, ","), strings.Join(after.Permissions, ",")},
		"tool_groups":     {strings.Join(before.ToolGroups, ","), strings.Join(after.ToolGroups, ",")},
		"notify_policy":   {before.NotifyPolicy, after.NotifyPolicy},
		"scope":           {string(before.Scope), string(after.Scope)},
	}
	for key, p := range pairs {
		if p[0] != p[1] {
			out[key] = map[string]any{"from": p[0], "to": p[1]}
		}
	}
	return out
}

/*
createParams are the insert of a new routine.
*/
func createParams(r models.OperatorRoutine, now time.Time) models.CreateOperatorRoutineParams {
	return models.CreateOperatorRoutineParams{
		WorkspaceID: r.WorkspaceID, Name: r.Name, Description: r.Description, TemplateKey: r.TemplateKey,
		Prompt: r.Prompt, Cadence: r.Cadence, CronExpr: r.CronExpr, RunAt: r.RunAt, Timezone: r.Timezone,
		NextRunAt: r.NextRunAt, CreatedByMemberID: r.CreatedByMemberID, OwnerMemberID: r.OwnerMemberID,
		Permissions: r.Permissions, ToolGroups: r.ToolGroups, InboxIds: r.InboxIds, WritePolicies: r.WritePolicies,
		Scope: r.Scope, Model: r.Model, Effort: r.Effort, MaxModelCalls: r.MaxModelCalls,
		MaxToolCalls: r.MaxToolCalls, MonthlyBudgetMicros: r.MonthlyBudgetMicros, NotifyPolicy: r.NotifyPolicy,
		Delivery: r.Delivery, Status: r.Status, Now: now,
	}
}

/*
updateParams are the update of a routine.
*/
func updateParams(r models.OperatorRoutine, now time.Time) models.UpdateOperatorRoutineParams {
	return models.UpdateOperatorRoutineParams{
		WorkspaceID: r.WorkspaceID, ID: r.ID, Name: r.Name, Description: r.Description, Prompt: r.Prompt,
		Cadence: r.Cadence, CronExpr: r.CronExpr, RunAt: r.RunAt, Timezone: r.Timezone, NextRunAt: r.NextRunAt,
		OwnerMemberID: r.OwnerMemberID, Permissions: r.Permissions, ToolGroups: r.ToolGroups, InboxIds: r.InboxIds,
		WritePolicies: r.WritePolicies, Scope: r.Scope, Model: r.Model, Effort: r.Effort,
		MaxModelCalls: r.MaxModelCalls, MaxToolCalls: r.MaxToolCalls, MonthlyBudgetMicros: r.MonthlyBudgetMicros,
		NotifyPolicy: r.NotifyPolicy, Delivery: r.Delivery, Status: r.Status, PauseReason: r.PauseReason,
		ConsecutiveFailures: r.ConsecutiveFailures, State: r.State, Now: now,
	}
}
