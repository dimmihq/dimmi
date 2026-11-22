/*
|--------------------------------------------------------------------------
| Routine Settings
|--------------------------------------------------------------------------
|
| A routine's settings are checked whole, after the input is laid over
| what was saved: its text, when it runs, what it may read and change,
| how far a run may go, and who it runs as. Nothing a routine is given
| may go beyond what the admin setting it up holds.
|
*/

package operatorchat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/cadence"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/ai/operator"
	"github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
apply lays the input over the routine and checks the result.
*/
func (a *Routines) apply(ctx context.Context, q *models.Queries, r *models.OperatorRoutine, in RoutineInput) error {
	if err := applyText(r, in); err != nil {
		return err
	}
	if err := a.applyTiming(r, in); err != nil {
		return err
	}
	if err := applyAccess(ctx, q, r, in); err != nil {
		return err
	}
	if err := applyModel(r, in); err != nil {
		return err
	}
	if err := applyLimits(r, in); err != nil {
		return err
	}
	if err := applyDelivery(ctx, q, r, in); err != nil {
		return err
	}
	if err := applyOwner(ctx, q, r, in); err != nil {
		return err
	}
	return a.applyStatus(r, in)
}

/*
applyText checks the name, description, prompt and delivery.
*/
func applyText(r *models.OperatorRoutine, in RoutineInput) error {
	if in.Name != nil {
		r.Name = strings.TrimSpace(*in.Name)
	}
	if n := len([]rune(r.Name)); n == 0 || n > 120 {
		return invalid("name_invalid", "A routine's name is 1 to 120 characters.", "name")
	}
	if in.Description != nil {
		r.Description = strings.TrimSpace(*in.Description)
	}
	if len([]rune(r.Description)) > 500 {
		return invalid("description_too_long", "A description is at most 500 characters.", "description")
	}
	if in.Prompt != nil {
		r.Prompt = strings.TrimSpace(*in.Prompt)
	}
	if n := len([]rune(r.Prompt)); n == 0 || n > 8000 {
		return invalid("prompt_invalid", "A routine's prompt is 1 to 8000 characters.", "prompt")
	}
	if in.NotifyPolicy != nil {
		r.NotifyPolicy = *in.NotifyPolicy
	}
	if !slices.Contains([]string{"on_findings", "always", "on_error_only"}, r.NotifyPolicy) {
		return invalid("notify_policy_invalid", "Notify on_findings, always or on_error_only.", "notify_policy")
	}
	return nil
}

/*
applyTiming checks when the routine runs and works out its next run.
*/
func (a *Routines) applyTiming(r *models.OperatorRoutine, in RoutineInput) error {
	timing := in.Cadence != nil || in.CronExpr != nil || in.RunAt != nil || in.Timezone != nil
	if in.Cadence != nil {
		r.Cadence = *in.Cadence
	}
	if in.CronExpr != nil {
		r.CronExpr = in.CronExpr
	}
	if in.Timezone != nil {
		r.Timezone = strings.TrimSpace(*in.Timezone)
	}
	r.RunAt = nil
	if r.Cadence == cadence.Once {
		r.RunAt, r.CronExpr = in.RunAt, nil
		if r.RunAt == nil && !timing {
			r.RunAt = r.NextRunAt
		}
	}
	s, err := operator.ScheduleOf(*r)
	switch {
	case errors.Is(err, cadence.ErrKind):
		return invalid("cadence_invalid", "Run daily, weekly, monthly, on a cron or once.", "cadence")
	case errors.Is(err, cadence.ErrTimezone):
		return invalid("timezone_invalid", "Use a time zone such as Europe/Berlin.", "timezone")
	case errors.Is(err, cadence.ErrRunAt):
		return invalid("run_at_required", "A one-off routine needs a time to run at.", "run_at")
	case err != nil:
		return invalid("cron_invalid", "The cron expression is not valid: "+err.Error(), "cron_expression")
	}
	now := a.Clock.Now()
	if s.CheckFrequency(now) != nil {
		return invalid("routine_too_frequent", "A routine runs at most once an hour.", "cron_expression")
	}
	if r.Cadence == cadence.Once && timing && !r.RunAt.After(now) {
		return invalid("run_at_past", "A one-off routine runs at a time still to come.", "run_at")
	}
	if timing || r.NextRunAt == nil {
		r.NextRunAt, _ = operator.NextRun(*r, now)
	}
	return nil
}

/*
repositoryName is owner/repo.
*/
var repositoryName = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,100}/[A-Za-z0-9_.-]{1,100}$`)

/*
applyAccess checks what the routine may read and change: permissions,
tool groups, inboxes, write policies and its scope, none beyond the
caller's own.
*/
func applyAccess(ctx context.Context, q *models.Queries, r *models.OperatorRoutine, in RoutineInput) error {
	m := in.Membership
	if in.Permissions != nil {
		r.Permissions = in.Permissions
	}
	if err := checkKnown(r.Permissions); err != nil {
		return err
	}
	if in.Permissions != nil {
		if err := checkHeld(m, in.Permissions); err != nil {
			return err
		}
	}
	if in.ToolGroups != nil {
		r.ToolGroups = in.ToolGroups
	}
	if err := checkGroups(r.ToolGroups); err != nil {
		return err
	}
	if in.InboxIDs != nil {
		r.InboxIds = in.InboxIDs
		if err := checkInboxes(ctx, q, m, r.InboxIds); err != nil {
			return err
		}
	}
	if err := applyScope(ctx, q, r, in); err != nil {
		return err
	}
	if slices.Contains(r.ToolGroups, operator.RoutineGroupGitHub) && operator.ScopeOf(r.Scope).GitHub == nil {
		return invalid("github_scope_required", "Pick the GitHub repositories the routine reads.", "scope.github")
	}
	return nil
}

/*
checkKnown refuses permissions that do not exist.
*/
func checkKnown(list []string) error {
	known := auth.RolePermissions(enums.RoleOwner)
	for _, p := range list {
		if !slices.Contains(known, p) {
			return invalid("permission_invalid", "There is no permission "+p+".", "permissions")
		}
	}
	return nil
}

/*
checkHeld refuses permissions the caller does not hold.
*/
func checkHeld(m authz.Membership, list []string) error {
	for _, p := range list {
		if !m.Can(p) {
			return invalid("permission_not_held", "You cannot give a routine "+p+", which you do not hold.",
				"permissions")
		}
	}
	return nil
}

/*
checkGroups refuses unknown tool groups.
*/
func checkGroups(list []string) error {
	for _, g := range list {
		if !slices.Contains(operator.RoutineGroups(), g) {
			return invalid("tool_group_invalid", "There is no tool group "+g+".", "tool_groups")
		}
	}
	return nil
}

/*
applyScope checks the write policies and the scope.
*/
func applyScope(ctx context.Context, q *models.Queries, r *models.OperatorRoutine, in RoutineInput) error {
	if in.WritePolicies != nil {
		if err := checkPolicies(in.WritePolicies); err != nil {
			return err
		}
		r.WritePolicies, _ = json.Marshal(in.WritePolicies)
	}
	if in.Scope != nil {
		if err := checkScope(ctx, q, in.Membership, *in.Scope); err != nil {
			return err
		}
		r.Scope, _ = json.Marshal(in.Scope)
	}
	return nil
}

/*
checkPolicies refuses write policies other than ask or off for a known
group.
*/
func checkPolicies(policies map[string]string) error {
	for g, p := range policies {
		if !operator.ValidGroup(g) || (p != string(operator.PolicyAsk) && p != string(operator.PolicyOff)) {
			return invalid("write_policy_invalid", "Each write policy is ask or off for a known group.",
				"write_policies")
		}
	}
	return nil
}

/*
checkInboxes refuses inboxes that do not exist or the caller cannot open.
*/
func checkInboxes(ctx context.Context, q *models.Queries, m authz.Membership, list []ids.UUID) error {
	if len(list) > 100 {
		return invalid("inboxes_too_many", "A routine names at most 100 inboxes.", "inbox_ids")
	}
	inboxes, err := q.ListInboxes(ctx, m.Workspace.ID)
	if err != nil {
		return fmt.Errorf("list inboxes: %w", err)
	}
	scope := m.ConversationScope()
	for _, id := range list {
		if !slices.ContainsFunc(inboxes, func(i models.Inbox) bool { return i.ID == id }) || !scope.OpensInbox(id) {
			return invalid("inbox_invalid", "Pick inboxes you can open.", "inbox_ids")
		}
	}
	return nil
}

/*
checkScope checks the GitHub connection and repositories a routine reads.
*/
func checkScope(ctx context.Context, q *models.Queries, m authz.Membership, s operator.RoutineScope) error {
	if len(s.HelpCenterIDs) > 20 {
		return invalid("help_centers_too_many", "A routine names at most 20 help centers.", "scope.help_center_ids")
	}
	g := s.GitHub
	if g == nil {
		return nil
	}
	if !m.Can("github:read") {
		return invalid("github_not_allowed", "Reading GitHub needs github:read.", "scope.github")
	}
	inst, err := q.FindIntegrationInstallation(ctx, models.FindIntegrationInstallationParams{
		WorkspaceID: m.Workspace.ID, ID: g.InstallationID,
	})
	if errors.Is(err, database.ErrNotFound) || (err == nil && (inst.Provider != "github" || inst.Status != "active")) {
		return invalid("github_installation_invalid", "Pick a connected GitHub account.",
			"scope.github.installation_id")
	}
	if err != nil {
		return fmt.Errorf("find installation: %w", err)
	}
	if len(g.Repositories) == 0 || len(g.Repositories) > maxRepositories {
		return invalid("repositories_invalid", "Pick 1 to 20 repositories.", "scope.github.repositories")
	}
	for _, repo := range g.Repositories {
		if !repositoryName.MatchString(repo) {
			return invalid("repositories_invalid", "Name repositories as owner/repo.", "scope.github.repositories")
		}
	}
	return nil
}

/*
applyModel checks the model and effort.
*/
func applyModel(r *models.OperatorRoutine, in RoutineInput) error {
	if in.Model != nil {
		r.Model = nil
		if model := strings.TrimSpace(*in.Model); model != "" {
			r.Model = &model
		}
	}
	if r.Model != nil && len(*r.Model) > 200 {
		return invalid("model_invalid", "Model names are at most 200 characters.", "model")
	}
	if in.Effort != nil {
		r.Effort = nil
		if *in.Effort != "" {
			r.Effort = in.Effort
		}
	}
	if r.Effort != nil && !slices.Contains([]string{"low", "medium", "high"}, *r.Effort) {
		return invalid("effort_invalid", "Effort is low, medium or high.", "effort")
	}
	return nil
}

/*
applyLimits checks the call limits and the monthly budget.
*/
func applyLimits(r *models.OperatorRoutine, in RoutineInput) error {
	if in.MaxModelCalls != nil {
		r.MaxModelCalls = *in.MaxModelCalls
	}
	if r.MaxModelCalls < 1 || r.MaxModelCalls > 30 {
		return invalid("max_model_calls_invalid", "A run makes 1 to 30 model calls.", "max_model_calls")
	}
	if in.MaxToolCalls != nil {
		r.MaxToolCalls = *in.MaxToolCalls
	}
	if r.MaxToolCalls < 1 || r.MaxToolCalls > 100 {
		return invalid("max_tool_calls_invalid", "A run makes 1 to 100 tool calls.", "max_tool_calls")
	}
	switch {
	case in.ClearBudget:
		r.MonthlyBudgetMicros = nil
	case in.MonthlyBudgetMicros != nil && *in.MonthlyBudgetMicros <= 0:
		return invalid("budget_invalid", "A monthly budget is more than zero.", "monthly_budget_micros")
	case in.MonthlyBudgetMicros != nil:
		r.MonthlyBudgetMicros = in.MonthlyBudgetMicros
	}
	return nil
}

/*
applyOwner hands the routine to another owner, who must hold all the
routine needs and may manage the AI.
*/
func applyOwner(ctx context.Context, q *models.Queries, r *models.OperatorRoutine, in RoutineInput) error {
	m := in.Membership
	if in.OwnerMemberID != nil {
		r.OwnerMemberID = in.OwnerMemberID
	}
	if r.OwnerMemberID == nil {
		return invalid("owner_required", "Pick the teammate the routine runs as.", "owner_member_id")
	}
	if *r.OwnerMemberID == m.Member.ID && in.OwnerMemberID == nil {
		return nil
	}
	owner, ok, err := operator.LoadMember(ctx, q, m.Workspace.ID, *r.OwnerMemberID)
	if err != nil {
		return err
	}
	if ok {
		_, ok, err = operator.Narrow(ctx, q, owner, *r)
	}
	if err != nil {
		return err
	}
	if !ok && (in.OwnerMemberID != nil || in.Permissions != nil || in.InboxIDs != nil) {
		return invalid("owner_invalid",
			"The owner must be an active teammate who manages the AI and holds every permission and inbox "+
				"the routine uses.", "owner_member_id")
	}
	return nil
}

/*
applyStatus pauses or resumes the routine and forgets its state when
asked. Resuming starts over: failures are forgotten and the next run is
worked out from now.
*/
func (a *Routines) applyStatus(r *models.OperatorRoutine, in RoutineInput) error {
	if in.ResetState {
		r.State = json.RawMessage("{}")
	}
	if in.Status == nil || *in.Status == r.Status {
		if r.Status != "active" {
			r.NextRunAt = nil
		}
		return nil
	}
	switch *in.Status {
	case "paused":
		r.Status, r.NextRunAt = "paused", nil
	case "active":
		r.Status, r.PauseReason, r.ConsecutiveFailures = "active", nil, 0
		r.NextRunAt, _ = operator.NextRun(*r, a.Clock.Now())
		if r.NextRunAt == nil {
			return invalid("routine_finished", "This one-off routine has no run left; pick a new time.", "run_at")
		}
	default:
		return invalid("status_invalid", "A routine is active or paused.", "status")
	}
	return nil
}

/*
deref reads an optional string.
*/
func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

/*
workspaceZone is the workspace's time zone, UTC when it has none.
*/
func workspaceZone(m authz.Membership) string {
	if m.Workspace.Timezone == "" {
		return "UTC"
	}
	return m.Workspace.Timezone
}

/*
invalid is a validation fault on one field.
*/
func invalid(code, message, field string) error {
	return fault.New(fault.Invalid, code, message).OnField(field)
}

/*
applyDelivery checks who the routine tells: active teammates of the
workspace, and its Slack destinations.
*/
func applyDelivery(ctx context.Context, q *models.Queries, r *models.OperatorRoutine, in RoutineInput) error {
	d := in.Delivery
	if d == nil {
		return nil
	}
	if len(d.InAppMemberIDs) > 100 || len(d.EmailMemberIDs) > 100 || len(d.SlackDestinationIDs) > 20 {
		return invalid("delivery_invalid", "Tell at most 100 teammates and 20 Slack destinations.", "delivery")
	}
	ws := in.Membership.Workspace.ID
	if err := checkMembers(ctx, q, ws, slices.Concat(d.InAppMemberIDs, d.EmailMemberIDs)); err != nil {
		return err
	}
	if err := checkDestinations(ctx, q, ws, d.SlackDestinationIDs); err != nil {
		return err
	}
	r.Delivery, _ = json.Marshal(d)
	return nil
}

/*
checkMembers refuses anyone who is not an active teammate of the
workspace.
*/
func checkMembers(ctx context.Context, q *models.Queries, ws ids.UUID, list []ids.UUID) error {
	for _, id := range list {
		member, err := q.FindWorkspaceMember(ctx, models.FindWorkspaceMemberParams{WorkspaceID: ws, ID: id})
		if errors.Is(err, database.ErrNotFound) || (err == nil && member.Status != string(enums.MemberStatusActive)) {
			return invalid("delivery_member_invalid", "Pick active teammates of this workspace.", "delivery")
		}
		if err != nil {
			return fmt.Errorf("find member: %w", err)
		}
	}
	return nil
}

/*
checkDestinations refuses anything but the workspace's Slack
destinations.
*/
func checkDestinations(ctx context.Context, q *models.Queries, ws ids.UUID, list []ids.UUID) error {
	for _, id := range list {
		row, err := q.FindNotificationDestination(ctx, models.FindNotificationDestinationParams{
			WorkspaceID: ws, ID: id,
		})
		if errors.Is(err, database.ErrNotFound) || (err == nil && !strings.HasPrefix(row.Type, "slack")) {
			return invalid("delivery_destination_invalid", "Pick Slack destinations of this workspace.", "delivery")
		}
		if err != nil {
			return fmt.Errorf("find destination: %w", err)
		}
	}
	return nil
}
