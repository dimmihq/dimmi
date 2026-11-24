/*
|--------------------------------------------------------------------------
| Routine Tools
|--------------------------------------------------------------------------
|
| What a routine keeps between runs and how it says it found nothing.
| Its state is a small JSON object, such as the last pull request it
| read, that only its own runs read and write; a dry run reads it but
| never moves it. Any run may list the suggestions made lately, with
| their keys and what became of them, so nothing is suggested twice.
|
*/

package operator

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/models"
)

/*
The limits of the routine tools: the state's size, and how far back and
how many suggestions a listing shows.
*/
const (
	maxRoutineState  = 64 << 10
	recentDaysNormal = 30
	recentDaysMax    = 90
	recentListed     = 50
)

/*
routineTools are the tools of a routine's run, and the listing of
recent suggestions every run may use.
*/
func (e *Engine) routineTools() []tool {
	return []tool{
		e.skillTool(),
		{spec: contracts.ToolSpec{
			Name:        "get_routine_state",
			Description: "Read what this routine kept from its earlier runs, such as cursors and what it covered.",
			Schema:      schema(`{"type":"object","properties":{}}`),
		}, handle: e.routineGetState, routine: true, mainOnly: true},
		{
			spec: contracts.ToolSpec{
				Name: "save_routine_state",
				Description: "Save what the next run of this routine needs, as a JSON object of at most 64 KB. " +
					"Keys given replace the saved ones; every other saved key stays.",
				Schema: schema(`{"type":"object","required":["state"],"properties":{` +
					`"state":{"type":"object","description":"What to save, such as {\"next_offset\": 15}."},` +
					`"replace_everything":{"type":"boolean","description":"true to drop every key saved before ` +
					`and keep only this object. Leave out to keep them."}}}`),
			}, handle: e.routineSetState, routine: true, mainOnly: true,
			was: map[string]string{"replace": "replace_everything"},
		},
		{spec: contracts.ToolSpec{
			Name: "report_nothing_to_change",
			Description: "Say that nothing needs the team's attention this run, so nobody is notified. " +
				"Ignored when this run already suggested changes.",
			Schema: schema(`{"type":"object","required":["reason"],"properties":{` +
				`"reason":{"type":"string","maxLength":500,"description":"One line on what you checked and ` +
				`why nothing needs changing."}}}`),
		}, handle: e.markNoop, routine: true, mainOnly: true},
		{
			spec: contracts.ToolSpec{
				Name: "list_recent_suggestions",
				Description: "List the changes Operator suggested lately, newest first, with their status (pending, " +
					"applied, declined, reverted, failed), dedup_key and operations, to avoid suggesting them again.",
				Schema: schema(`{"type":"object","properties":{` +
					`"days_back":{"type":"integer","minimum":1,"maximum":90,"description":"How many days back to ` +
					`look; 30 by default."},` +
					`"status":{"type":"string","enum":["pending","applied","declined","reverted","failed"],` +
					`"description":"Only suggestions with this status; leave out for all."},` +
					`"only_this_routine":{"type":"boolean","description":"true for only this routine's ` +
					`suggestions."}}}`),
			}, handle: e.listRecentChangesets,
			was: map[string]string{"since_days": "days_back", "routine_only": "only_this_routine"},
		},
	}
}

/*
routineGetState returns the routine's state.
*/
func (e *Engine) routineGetState(_ context.Context, s *session, _ json.RawMessage) (result, error) {
	state := s.routine.row.State
	if len(state) == 0 {
		state = json.RawMessage("{}")
	}
	return result{text: string(state), step: Step{Tool: "get_routine_state", Summary: "Read the routine's state"}}, nil
}

/*
routineSetState merges keys into the routine's state, or replaces it;
a dry run only says what it would save.
*/
func (e *Engine) routineSetState(ctx context.Context, s *session, args json.RawMessage) (result, error) {
	var in struct {
		State   map[string]json.RawMessage `json:"state"`
		Replace bool                       `json:"replace_everything"`
	}
	if err := decode(args, &in); err != nil {
		return result{}, err
	}
	if in.State == nil {
		return result{}, fmt.Errorf("%w: state is a JSON object", errBadArguments)
	}
	next := map[string]json.RawMessage{}
	if !in.Replace {
		_ = json.Unmarshal(s.routine.row.State, &next)
	}
	maps.Copy(next, in.State)
	encoded, err := json.Marshal(next)
	if err != nil || len(encoded) > maxRoutineState {
		return result{}, fmt.Errorf("%w: the state is a JSON object of at most 64 KB", errBadArguments)
	}
	step := Step{Tool: "save_routine_state", Summary: "Saved the routine's state"}
	if s.routine.dryRun {
		step.Summary = "Would save the routine's state (dry run)"
		return result{text: "Saved for this dry run only: a dry run never keeps state, which is expected. " +
			"Carry on as if it was saved.", step: step}, nil
	}
	err = e.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		return models.New(tx).SetOperatorRoutineState(ctx, models.SetOperatorRoutineStateParams{
			WorkspaceID: s.routine.row.WorkspaceID, ID: s.routine.row.ID, State: encoded, Now: e.Clock.Now(),
		})
	})
	if err != nil {
		return result{}, fmt.Errorf("save routine state: %w", err)
	}
	s.routine.row.State = encoded
	return result{text: "Saved.", step: step}, nil
}

/*
markNoop records that the run found nothing needing attention.
*/
func (e *Engine) markNoop(_ context.Context, s *session, args json.RawMessage) (result, error) {
	var in struct {
		Reason string `json:"reason"`
	}
	if err := decode(args, &in); err != nil {
		return result{}, err
	}
	reason := truncate(strings.TrimSpace(in.Reason), 500)
	if len(s.proposed) > 0 {
		return result{
			text: "Ignored: this run already suggested changes, so the team is told.",
			step: Step{Tool: "report_nothing_to_change", Summary: "Kept the findings"},
		}, nil
	}
	s.routine.noop, s.routine.noopReason = true, reason
	return result{
		text: "Noted: nothing needs attention.",
		step: Step{Tool: "report_nothing_to_change", Summary: "Nothing needs attention: " + truncate(reason, 80)},
	}, nil
}

/*
listRecentChangesets lists the suggestions the session may see that
were made lately.
*/
func (e *Engine) listRecentChangesets(ctx context.Context, s *session, args json.RawMessage) (result, error) {
	var in struct {
		SinceDays   int    `json:"days_back"`
		Status      string `json:"status"`
		RoutineOnly bool   `json:"only_this_routine"`
	}
	if err := decode(args, &in); err != nil {
		return result{}, err
	}
	days := in.SinceDays
	if days <= 0 {
		days = recentDaysNormal
	}
	since := e.Clock.Now().Add(-time.Duration(min(days, recentDaysMax)) * 24 * time.Hour)
	f := ChangesetFilter{Since: &since, WithoutPreviews: true, Limit: recentListed}
	if in.Status != "" {
		f.Status = &in.Status
	}
	if in.RoutineOnly && s.routine != nil {
		f.Routine = &s.routine.row.ID
	}
	var rows []models.OperatorChangeset
	err := e.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		rows, err = models.New(tx).ListOperatorChangesets(ctx, ChangesetParams(s.m, f))
		return err
	})
	if err != nil {
		return result{}, fmt.Errorf("list changesets: %w", err)
	}
	var b strings.Builder
	for _, c := range rows {
		fmt.Fprintf(&b, "- %s · %s · %s · %s", models.PublicID(models.PrefixOperatorChangeset, c.ID), c.Status,
			c.CreatedAt.UTC().Format("2006-01-02"), c.Title)
		if c.DedupKey != nil {
			fmt.Fprintf(&b, " · key %s", *c.DedupKey)
		}
		for _, op := range Operations(c.Operations) {
			b.WriteString("\n  - " + op.Label)
		}
		b.WriteString("\n")
	}
	if len(rows) == 0 {
		fmt.Fprintf(&b, "No changes were suggested in the last %d days.", days)
	}
	return result{text: b.String(), step: Step{
		Tool: "list_recent_suggestions", Summary: fmt.Sprintf("Listed %d recent suggestions", len(rows)),
	}}, nil
}

/*
ChangesetFilter narrows a listing of changesets: by conversation,
thread, routine or status, to those made since a moment, without dry
run previews, and how many.
*/
type ChangesetFilter struct {
	Conversation    *ids.UUID
	Thread          *ids.UUID
	Routine         *ids.UUID
	Status          *string
	Since           *time.Time
	WithoutPreviews bool
	Limit           int32
}

/*
ChangesetParams are the listing's query arguments for the teammate.
*/
func ChangesetParams(m authz.Membership, f ChangesetFilter) models.ListOperatorChangesetsParams {
	scope := m.ConversationScope().Params()
	vis := RoutineVisibilityOf(m)
	limit := f.Limit
	if limit <= 0 {
		limit = 100
	}
	return models.ListOperatorChangesetsParams{
		WorkspaceID: m.Workspace.ID, MemberID: &m.Member.ID, ConversationID: f.Conversation, ThreadID: f.Thread,
		RoutineID: f.Routine, Status: f.Status, Since: f.Since, WithoutPreviews: f.WithoutPreviews,
		ReadsRoutines: vis.Reads, SeesEverything: vis.SeesEverything, SeesAllModes: vis.SeesAllModes,
		AccessMode: scope.Mode, AccessViewer: scope.Viewer, AccessTeams: scope.Teams,
		AccessUnassigned: scope.Unassigned, AccessHiddenInboxes: scope.HiddenInboxes, RowLimit: limit,
	}
}
