/*
|--------------------------------------------------------------------------
| Operator Routine Controller
|--------------------------------------------------------------------------
|
| Admins put Operator on a clock here: the templates routines start
| from, the routines themselves, and their runs, started now or by the
| clock. The action decides who may do what; this maps ids and shapes.
|
*/

package v1

import (
	"context"

	"github.com/dimmihq/dimmi/framework/fault"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/operatorchat"
	"github.com/dimmihq/dimmi/app/ai/operator"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
OperatorRoutinesOutput is the workspace's routines.
*/
type OperatorRoutinesOutput struct {
	Body fhttp.Collection[resources.OperatorRoutine]
}

/*
OperatorRoutineOutput is one routine.
*/
type OperatorRoutineOutput struct {
	Body fhttp.Resource[resources.OperatorRoutine]
}

/*
OperatorRoutineController serves Operator's routines.
*/
type OperatorRoutineController struct {
	Routines *operatorchat.Routines
	IDs      *ids.Registry
}

/*
Templates lists the templates a routine starts from.
*/
func (c *OperatorRoutineController) Templates(ctx context.Context, _ *requests.NoInput,
) (*OperatorRoutineTemplatesOutput, error) {
	list, err := c.Routines.Templates(membership(ctx))
	if err != nil {
		return nil, err
	}
	items := resources.NewOperatorRoutineTemplates(list)
	page := pagination.Page[resources.OperatorRoutineTemplate]{Items: items, Limit: len(items)}
	return &OperatorRoutineTemplatesOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
Index lists the workspace's routines, of filter[status] when sent.
*/
func (c *OperatorRoutineController) Index(ctx context.Context, _ *requests.NoInput,
) (*OperatorRoutinesOutput, error) {
	r, _ := fhttp.RequestFrom(ctx)
	var status *string
	if s := r.URL.Query().Get("filter[status]"); s != "" {
		status = &s
	}
	rows, err := c.Routines.List(ctx, membership(ctx), status)
	if err != nil {
		return nil, err
	}
	page := pagination.Page[resources.OperatorRoutine]{Items: resources.NewOperatorRoutines(rows), Limit: 200}
	return &OperatorRoutinesOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
Store creates a routine.
*/
func (c *OperatorRoutineController) Store(ctx context.Context, in *requests.StoreOperatorRoutineRequest,
) (*OperatorRoutineOutput, error) {
	input, err := c.input(membership(ctx), in.Body.OperatorRoutineBody)
	if err != nil {
		return nil, err
	}
	input.TemplateKey = in.Body.TemplateKey
	view, err := c.Routines.Create(ctx, input)
	if err != nil {
		return nil, err
	}
	return &OperatorRoutineOutput{Body: fhttp.NewResource(resources.NewOperatorRoutine(view))}, nil
}

/*
Show returns one routine.
*/
func (c *OperatorRoutineController) Show(ctx context.Context, in *requests.OperatorRoutineRequest,
) (*OperatorRoutineOutput, error) {
	id, err := c.decode(models.PrefixOperatorRoutine, in.Routine)
	if err != nil {
		return nil, err
	}
	view, err := c.Routines.Show(ctx, membership(ctx), id)
	if err != nil {
		return nil, err
	}
	return &OperatorRoutineOutput{Body: fhttp.NewResource(resources.NewOperatorRoutine(view))}, nil
}

/*
Update changes a routine.
*/
func (c *OperatorRoutineController) Update(ctx context.Context, in *requests.UpdateOperatorRoutineRequest,
) (*OperatorRoutineOutput, error) {
	id, err := c.decode(models.PrefixOperatorRoutine, in.Routine)
	if err != nil {
		return nil, err
	}
	input, err := c.input(membership(ctx), in.Body.OperatorRoutineBody)
	if err != nil {
		return nil, err
	}
	input.Status, input.ClearBudget, input.ResetState = in.Body.Status, in.Body.ClearBudget, in.Body.ResetState
	view, err := c.Routines.Update(ctx, id, input)
	if err != nil {
		return nil, err
	}
	return &OperatorRoutineOutput{Body: fhttp.NewResource(resources.NewOperatorRoutine(view))}, nil
}

/*
Destroy deletes a routine.
*/
func (c *OperatorRoutineController) Destroy(ctx context.Context, in *requests.OperatorRoutineRequest,
) (*struct{}, error) {
	id, err := c.decode(models.PrefixOperatorRoutine, in.Routine)
	if err != nil {
		return nil, err
	}
	return &struct{}{}, c.Routines.Delete(ctx, membership(ctx), id)
}

/*
StoreRun starts a run of the routine now.
*/
func (c *OperatorRoutineController) StoreRun(ctx context.Context, in *requests.StoreOperatorRoutineRunRequest,
) (*OperatorRoutineRunOutput, error) {
	id, err := c.decode(models.PrefixOperatorRoutine, in.Routine)
	if err != nil {
		return nil, err
	}
	start := c.Routines.Run
	if in.Body.DryRun {
		start = c.Routines.DryRun
	}
	run, err := start(ctx, membership(ctx), id)
	if err != nil {
		return nil, err
	}
	return &OperatorRoutineRunOutput{Body: fhttp.NewResource(resources.NewOperatorRoutineRun(run))}, nil
}

/*
Runs lists a routine's runs.
*/
func (c *OperatorRoutineController) Runs(ctx context.Context, in *requests.OperatorRoutineRequest,
) (*OperatorRoutineRunsOutput, error) {
	id, err := c.decode(models.PrefixOperatorRoutine, in.Routine)
	if err != nil {
		return nil, err
	}
	rows, err := c.Routines.Runs(ctx, membership(ctx), id)
	if err != nil {
		return nil, err
	}
	page := pagination.Page[resources.OperatorRoutineRun]{Items: resources.NewOperatorRoutineRuns(rows), Limit: 100}
	return &OperatorRoutineRunsOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
ShowRun returns one run of a routine.
*/
func (c *OperatorRoutineController) ShowRun(ctx context.Context, in *requests.OperatorRoutineRunRequest,
) (*OperatorRoutineRunOutput, error) {
	routine, err := c.decode(models.PrefixOperatorRoutine, in.Routine)
	if err != nil {
		return nil, err
	}
	run, err := c.decode(models.PrefixOperatorRoutineRun, in.Run)
	if err != nil {
		return nil, err
	}
	row, err := c.Routines.ShowRun(ctx, membership(ctx), routine, run)
	if err != nil {
		return nil, err
	}
	return &OperatorRoutineRunOutput{Body: fhttp.NewResource(resources.NewOperatorRoutineRun(row))}, nil
}

/*
input maps a routine's settings to the action's input.
*/
func (c *OperatorRoutineController) input(m authz.Membership, b requests.OperatorRoutineBody,
) (operatorchat.RoutineInput, error) {
	in := operatorchat.RoutineInput{
		Membership: m, Name: b.Name, Description: b.Description, Prompt: b.Prompt, Cadence: b.Cadence,
		CronExpr: b.CronExpression, RunAt: b.RunAt, Timezone: b.Timezone, Permissions: b.Permissions,
		ToolGroups: b.ToolGroups, WritePolicies: b.WritePolicies, Model: b.Model, Effort: b.Effort,
		MaxModelCalls: b.MaxModelCalls, MaxToolCalls: b.MaxToolCalls, MonthlyBudgetMicros: b.MonthlyBudgetMicros,
		NotifyPolicy: b.NotifyPolicy,
	}
	var err error
	if in.OwnerMemberID, err = decodeField(c.IDs, models.PrefixMember, b.OwnerID,
		invalidField("owner_id", "No such teammate.")); err != nil {
		return in, err
	}
	if b.InboxIDs != nil {
		if in.InboxIDs, err = c.decodeAll(models.PrefixInbox, b.InboxIDs, "inbox_ids"); err != nil {
			return in, err
		}
	}
	if b.Scope != nil {
		if in.Scope, err = c.scope(*b.Scope); err != nil {
			return in, err
		}
	}
	if b.Delivery != nil {
		if in.Delivery, err = c.delivery(*b.Delivery); err != nil {
			return in, err
		}
	}
	return in, nil
}

/*
scope maps a routine's scope to row ids.
*/
func (c *OperatorRoutineController) scope(s requests.OperatorRoutineScopeInput) (*operator.RoutineScope, error) {
	out := &operator.RoutineScope{}
	var err error
	out.HelpCenterIDs, err = c.decodeAll(models.PrefixHelpCenter, s.HelpCenterIDs, "scope.help_center_ids")
	if err != nil {
		return nil, err
	}
	if g := s.GitHub; g != nil {
		inst, err := c.IDs.Decode(string(models.PrefixIntegrationInstallation), g.InstallationID)
		if err != nil {
			return nil, invalidField("scope.github.installation_id", "No such GitHub connection.")
		}
		out.GitHub = &operator.GitHubScope{
			InstallationID: inst, Repositories: g.Repositories, BaseBranch: g.BaseBranch, IgnoreLabels: g.IgnoreLabels,
		}
	}
	return out, nil
}

/*
delivery maps who a routine tells to row ids.
*/
func (c *OperatorRoutineController) delivery(d requests.OperatorRoutineDeliveryInput,
) (*operator.RoutineDelivery, error) {
	out := &operator.RoutineDelivery{}
	var err error
	if out.InAppMemberIDs, err = c.decodeAll(models.PrefixMember, d.InAppMemberIDs, "delivery"); err != nil {
		return nil, err
	}
	if out.EmailMemberIDs, err = c.decodeAll(models.PrefixMember, d.EmailMemberIDs, "delivery"); err != nil {
		return nil, err
	}
	out.SlackDestinationIDs, err = c.decodeAll(models.PrefixDestination, d.SlackDestinationIDs, "delivery")
	return out, err
}

/*
decodeAll reads a list of public ids, refusing the field on any that is
not one.
*/
func (c *OperatorRoutineController) decodeAll(prefix models.Prefix, list []string, field string,
) ([]ids.UUID, error) {
	out := make([]ids.UUID, 0, len(list))
	for _, public := range list {
		id, err := c.IDs.Decode(string(prefix), public)
		if err != nil {
			return nil, invalidField(field, "No such "+string(prefix)+" id: "+public+".")
		}
		out = append(out, id)
	}
	return out, nil
}

/*
decode reads a public id; anything else is not found.
*/
func (c *OperatorRoutineController) decode(prefix models.Prefix, public string) (ids.UUID, error) {
	id, err := c.IDs.Decode(string(prefix), public)
	if err != nil {
		return ids.UUID{}, exceptions.ErrNotFound
	}
	return id, nil
}

/*
invalidField is a validation fault naming the field at fault.
*/
func invalidField(field, message string) error {
	return fault.New(fault.Invalid, "id_invalid", message).OnField(field)
}
