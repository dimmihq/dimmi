/*
|--------------------------------------------------------------------------
| Operator Routines
|--------------------------------------------------------------------------
|
| Admins put Operator on a clock here: they create routines from a
| template or from scratch, change, pause, resume and delete them, and
| run them now, for real or as a dry run. A routine never holds more
| than the admin who set it up: every permission it is given and every
| inbox it names must be the admin's own, and its owner must keep them
| for it to run. Every change lands in the audit log.
|
*/

package operatorchat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/audit"
	"github.com/dimmihq/dimmi/app/ai/operator"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
The limits of routines: how many a list shows, how many runs a history
shows, and how many repositories one reads.
*/
const (
	routinesListed  = 200
	runsListed      = 100
	maxRepositories = 20
	nextRunsShown   = 3
)

/*
Routines manages Operator's routines and their runs.

Rules:
  - Reading needs ai:read; changing, running and deleting need
    ai_agent:manage.
  - Every permission a routine is given and every inbox it names must be
    the caller's own; reading GitHub needs github:read.
  - A routine runs at most once an hour, and one live run at a time.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], a [fault]
for invalid input, [operator.ErrRoutineRunning], or a wrapped database
error.
*/
type Routines struct {
	DB    *database.DB
	Clock clock.Clock
	Audit *audit.RecordAuditLog
	Push  operator.PushRun
}

/*
RoutineInput is a routine as the caller wants it. On an update, nil
fields stay as they are.
*/
type RoutineInput struct {
	Membership          authz.Membership
	Name                *string
	Description         *string
	TemplateKey         *string
	Prompt              *string
	Cadence             *string
	CronExpr            *string
	RunAt               *time.Time
	Timezone            *string
	OwnerMemberID       *ids.UUID
	Permissions         []string
	ToolGroups          []string
	InboxIDs            []ids.UUID
	WritePolicies       map[string]string
	Scope               *operator.RoutineScope
	Model               *string
	Effort              *string
	MaxModelCalls       *int32
	MaxToolCalls        *int32
	MonthlyBudgetMicros *int64
	ClearBudget         bool
	NotifyPolicy        *string
	Delivery            *operator.RoutineDelivery
	Status              *string
	ResetState          bool
}

/*
RoutineView is a routine with when it runs next, what it spent this
month, and how many of its suggestions are in each status.
*/
type RoutineView struct {
	Routine     models.OperatorRoutine
	NextRuns    []time.Time
	MonthSpend  int64
	Suggestions map[string]int64
}

/*
Templates lists the routine templates.
*/
func (a *Routines) Templates(m authz.Membership) ([]operator.RoutineTemplate, error) {
	if !m.Can("ai:read") {
		return nil, exceptions.ErrForbidden
	}
	return operator.RoutineTemplates(), nil
}

/*
List returns the workspace's routines, newest first, of one status when
asked, each with its next runs and what it spent this month.
*/
func (a *Routines) List(ctx context.Context, m authz.Membership, status *string) ([]RoutineView, error) {
	if !m.Can("ai:read") {
		return nil, exceptions.ErrForbidden
	}
	var out []RoutineView
	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		rows, err := q.ListOperatorRoutines(ctx, models.ListOperatorRoutinesParams{
			WorkspaceID: m.Workspace.ID, Status: status, RowLimit: routinesListed,
		})
		if err != nil {
			return err
		}
		spend, err := a.monthSpend(ctx, q, m.Workspace.ID)
		if err != nil {
			return err
		}
		counts, err := suggestionCounts(ctx, q, m.Workspace.ID)
		if err != nil {
			return err
		}
		for _, r := range rows {
			v := a.schedule(r)
			v.MonthSpend, v.Suggestions = spend[r.ID], counts[r.ID]
			out = append(out, v)
		}
		return nil
	})
	return out, err
}

/*
monthSpend is what each routine of the workspace spent this month.
*/
func (a *Routines) monthSpend(ctx context.Context, q *models.Queries, ws ids.UUID) (map[ids.UUID]int64, error) {
	now := a.Clock.Now().UTC()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	rows, err := q.ListOperatorRoutineCosts(ctx, models.ListOperatorRoutineCostsParams{
		WorkspaceID: ws, Since: start, Until: start.AddDate(0, 1, 0), RowLimit: routinesListed,
	})
	if err != nil {
		return nil, fmt.Errorf("sum routine costs: %w", err)
	}
	out := make(map[ids.UUID]int64, len(rows))
	for _, r := range rows {
		out[r.ID] = r.CostMicros
	}
	return out, nil
}

/*
Show returns one routine with its next runs and this month's spend.
*/
func (a *Routines) Show(ctx context.Context, m authz.Membership, id ids.UUID) (RoutineView, error) {
	if !m.Can("ai:read") {
		return RoutineView{}, exceptions.ErrForbidden
	}
	var out RoutineView
	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		r, err := findRoutine(ctx, q, m, id)
		if err != nil {
			return err
		}
		out, err = a.view(ctx, q, r)
		return err
	})
	return out, err
}

/*
view reads a routine's next runs and this month's spend.
*/
func (a *Routines) view(ctx context.Context, q *models.Queries, r models.OperatorRoutine) (RoutineView, error) {
	out := a.schedule(r)
	now := a.Clock.Now().UTC()
	spend, err := q.SumOperatorRoutineCost(ctx, models.SumOperatorRoutineCostParams{
		WorkspaceID: r.WorkspaceID, RoutineID: r.ID, Since: time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC),
	})
	out.MonthSpend = spend
	if err != nil {
		return out, err
	}
	counts, err := suggestionCounts(ctx, q, r.WorkspaceID)
	out.Suggestions = counts[r.ID]
	return out, err
}

/*
suggestionCounts is how many suggestions each routine of the workspace
has in each status.
*/
func suggestionCounts(ctx context.Context, q *models.Queries, ws ids.UUID) (map[ids.UUID]map[string]int64, error) {
	rows, err := q.CountOperatorRoutineChangesets(ctx, ws)
	if err != nil {
		return nil, fmt.Errorf("count routine suggestions: %w", err)
	}
	out := map[ids.UUID]map[string]int64{}
	for _, row := range rows {
		if out[row.RoutineID] == nil {
			out[row.RoutineID] = map[string]int64{}
		}
		out[row.RoutineID][row.Status] = row.ChangesetsCount
	}
	return out, nil
}

/*
schedule is a routine with its next three runs.
*/
func (a *Routines) schedule(r models.OperatorRoutine) RoutineView {
	out := RoutineView{Routine: r}
	if r.Status == "active" && r.NextRunAt != nil {
		if s, err := operator.ScheduleOf(r); err == nil {
			out.NextRuns = append([]time.Time{*r.NextRunAt}, s.NextN(*r.NextRunAt, nextRunsShown-1)...)
		}
	}
	return out
}

/*
Create stores a new routine, filled from its template where the input
says nothing, owned by the caller unless another owner is named.
*/
func (a *Routines) Create(ctx context.Context, in RoutineInput) (RoutineView, error) {
	m := in.Membership
	if !m.Can("ai_agent:manage") {
		return RoutineView{}, exceptions.ErrForbidden
	}
	r, err := a.fromTemplate(in)
	if err != nil {
		return RoutineView{}, err
	}
	r.WorkspaceID, r.Status, r.Timezone = m.Workspace.ID, "active", workspaceZone(m)
	r.CreatedByMemberID, r.OwnerMemberID = &m.Member.ID, &m.Member.ID
	var out RoutineView
	err = a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		if err := a.apply(ctx, q, &r, in); err != nil {
			return err
		}
		row, err := q.CreateOperatorRoutine(ctx, createParams(r, a.Clock.Now()))
		if err != nil {
			return fmt.Errorf("create routine: %w", err)
		}
		if err := a.record(ctx, m, row, routineEvent{name: "operator.routine_created"}); err != nil {
			return err
		}
		out, err = a.view(ctx, q, row)
		return err
	})
	return out, err
}

/*
fromTemplate is a new routine filled from the template the input names,
or the custom one.
*/
func (a *Routines) fromTemplate(in RoutineInput) (models.OperatorRoutine, error) {
	key := operator.TemplateCustom
	if in.TemplateKey != nil {
		key = *in.TemplateKey
	}
	t, ok := operator.RoutineTemplateByKey(key)
	if !ok {
		return models.OperatorRoutine{}, invalid("template_invalid", "There is no such template.", "template_key")
	}
	r := models.OperatorRoutine{
		Name: t.Name, Prompt: t.Prompt, Cadence: t.Cadence, CronExpr: &t.Expr, Permissions: abilitiesOf(in.Membership),
		ToolGroups: operator.RoutineGroups(), InboxIds: []ids.UUID{}, WritePolicies: json.RawMessage("{}"),
		Scope: json.RawMessage("{}"), MaxModelCalls: 16, MaxToolCalls: 75, NotifyPolicy: t.NotifyPolicy,
		Delivery: json.RawMessage("{}"), State: json.RawMessage("{}"), MaxConsecutiveFailures: 3,
	}
	if key != operator.TemplateCustom {
		r.TemplateKey = &key
	}
	return r, nil
}

/*
Update changes a routine: its settings, its owner, pausing or resuming
it, or forgetting what it kept between runs.
*/
func (a *Routines) Update(ctx context.Context, id ids.UUID, in RoutineInput) (RoutineView, error) {
	m := in.Membership
	if !m.Can("ai_agent:manage") {
		return RoutineView{}, exceptions.ErrForbidden
	}
	var out RoutineView
	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		r, err := findRoutine(ctx, q, m, id)
		if err != nil {
			return err
		}
		before := r
		if err := a.apply(ctx, q, &r, in); err != nil {
			return err
		}
		row, err := q.UpdateOperatorRoutine(ctx, updateParams(r, a.Clock.Now()))
		if err != nil {
			return fmt.Errorf("update routine: %w", err)
		}
		event := "operator.routine_updated"
		switch {
		case before.Status != "paused" && row.Status == "paused":
			event = "operator.routine_paused"
		case before.Status != "active" && row.Status == "active":
			event = "operator.routine_resumed"
		}
		if err := a.record(ctx, m, row, routineEvent{name: event, changes: changed(before, row)}); err != nil {
			return err
		}
		out, err = a.view(ctx, q, row)
		return err
	})
	return out, err
}

/*
Delete stops a routine for good; its threads and suggestions stay.
*/
func (a *Routines) Delete(ctx context.Context, m authz.Membership, id ids.UUID) error {
	if !m.Can("ai_agent:manage") {
		return exceptions.ErrForbidden
	}
	return a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		r, err := findRoutine(ctx, q, m, id)
		if err != nil {
			return err
		}
		if _, err := q.DeleteOperatorRoutine(ctx, models.DeleteOperatorRoutineParams{
			WorkspaceID: m.Workspace.ID, ID: id, Now: new(a.Clock.Now()),
		}); err != nil {
			return fmt.Errorf("delete routine: %w", err)
		}
		return a.record(ctx, m, r, routineEvent{name: "operator.routine_deleted"})
	})
}

/*
Run queues a live run of the routine now; it is refused while another
live run is going.
*/
func (a *Routines) Run(ctx context.Context, m authz.Membership, id ids.UUID) (models.OperatorRoutineRun, error) {
	return a.queue(ctx, m, id, operator.TriggerManual)
}

/*
DryRun queues a dry run of the routine now: it reads real data, but its
suggestions are previews and it keeps no state.
*/
func (a *Routines) DryRun(ctx context.Context, m authz.Membership, id ids.UUID) (models.OperatorRoutineRun, error) {
	return a.queue(ctx, m, id, operator.TriggerDryRun)
}

/*
queue queues a run with the trigger.
*/
func (a *Routines) queue(ctx context.Context, m authz.Membership, id ids.UUID, trigger string,
) (models.OperatorRoutineRun, error) {
	if !m.Can("ai_agent:manage") {
		return models.OperatorRoutineRun{}, exceptions.ErrForbidden
	}
	var out models.OperatorRoutineRun
	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		r, err := findRoutine(ctx, q, m, id)
		if err != nil {
			return err
		}
		if trigger != operator.TriggerDryRun {
			if err := idle(ctx, q, r); err != nil {
				return err
			}
		}
		out, err = q.CreateOperatorRoutineRun(ctx, models.CreateOperatorRoutineRunParams{
			WorkspaceID: r.WorkspaceID, RoutineID: r.ID, Trigger: trigger, TriggeredByMemberID: &m.Member.ID,
			Status: "queued", Now: a.Clock.Now(),
		})
		if err != nil {
			return fmt.Errorf("queue routine run: %w", err)
		}
		if err := a.record(ctx, m, r, routineEvent{
			name: "operator.routine_triggered", changes: map[string]any{"trigger": trigger},
		}); err != nil {
			return err
		}
		return a.Push(ctx, tx, r.WorkspaceID, out.ID)
	})
	return out, err
}

/*
idle refuses a live run while another is going.
*/
func idle(ctx context.Context, q *models.Queries, r models.OperatorRoutine) error {
	_, err := q.FindActiveOperatorRoutineRun(ctx, models.FindActiveOperatorRoutineRunParams{
		WorkspaceID: r.WorkspaceID, RoutineID: r.ID,
	})
	if err == nil {
		return operator.ErrRoutineRunning
	}
	if !errors.Is(err, database.ErrNotFound) {
		return fmt.Errorf("find live routine run: %w", err)
	}
	return nil
}

/*
Runs returns a routine's runs, newest first.
*/
func (a *Routines) Runs(ctx context.Context, m authz.Membership, id ids.UUID) ([]models.OperatorRoutineRun, error) {
	if !m.Can("ai:read") {
		return nil, exceptions.ErrForbidden
	}
	var out []models.OperatorRoutineRun
	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		if _, err := findRoutine(ctx, q, m, id); err != nil {
			return err
		}
		var err error
		out, err = q.ListOperatorRoutineRuns(ctx, models.ListOperatorRoutineRunsParams{
			WorkspaceID: m.Workspace.ID, RoutineID: id, RowLimit: runsListed,
		})
		return err
	})
	return out, err
}

/*
ShowRun returns one run of a routine.
*/
func (a *Routines) ShowRun(ctx context.Context, m authz.Membership, routine, run ids.UUID,
) (models.OperatorRoutineRun, error) {
	if !m.Can("ai:read") {
		return models.OperatorRoutineRun{}, exceptions.ErrForbidden
	}
	var out models.OperatorRoutineRun
	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		if _, err := findRoutine(ctx, q, m, routine); err != nil {
			return err
		}
		var err error
		out, err = q.FindOperatorRoutineRun(ctx, models.FindOperatorRoutineRunParams{
			WorkspaceID: m.Workspace.ID, RoutineID: routine, ID: run,
		})
		if errors.Is(err, database.ErrNotFound) {
			return exceptions.ErrNotFound
		}
		return err
	})
	return out, err
}

/*
findRoutine returns one routine of the caller's workspace.
*/
func findRoutine(ctx context.Context, q *models.Queries, m authz.Membership, id ids.UUID,
) (models.OperatorRoutine, error) {
	r, err := q.FindOperatorRoutine(ctx, models.FindOperatorRoutineParams{WorkspaceID: m.Workspace.ID, ID: id})
	if errors.Is(err, database.ErrNotFound) {
		return r, exceptions.ErrNotFound
	}
	if err != nil {
		return r, fmt.Errorf("find routine: %w", err)
	}
	return r, nil
}

/*
abilitiesOf is what a routine runs with: every routine ability its
creator holds.
*/
func abilitiesOf(m authz.Membership) []string {
	out := []string{}
	for _, p := range operator.RoutineAbilities() {
		if m.Can(p) {
			out = append(out, p)
		}
	}
	return out
}
