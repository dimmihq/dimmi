/*
|--------------------------------------------------------------------------
| Workflow Tools
|--------------------------------------------------------------------------
|
| Operator reads workflows the way the builder shows them: the working
| copy as an outline of steps and paths with their ids, and everything
| publishing would refuse or warn about. It explains a conversation's
| automation from what was recorded: each live workflow's verdict on
| each event, and the steps each run took. Its changes are suggestions
| on the working copy (propose_automation.go); it never publishes.
|
*/

package operator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/workflowspec"
)

/*
The pages of workflows and series, by the path under the workspace.
*/
const (
	linkWorkflow = "workflows"
	linkSeries   = "series"
)

/*
How much of a conversation's automation one explanation reads.
*/
const (
	matchLogsRead = 10
	runsRead      = 10
)

/*
workflowTools are the tools that read workflows and explain their runs.
*/
func (e *Engine) workflowTools() []tool {
	if e.Services.Workflows == nil {
		return nil
	}
	return []tool{
		{spec: contracts.ToolSpec{
			Name: "list_workflows",
			Description: "List the workspace's workflows in priority order: when several want the same " +
				"conversation, the first wins. Each line has the workflow's id, trigger, status and link.",
			Schema: schema(`{"type":"object","properties":{}}`),
		}, handle: e.listWorkflows},
		{spec: contracts.ToolSpec{
			Name: "get_workflow_by_id",
			Description: "Read one workflow: its working copy as an outline of steps and paths with their " +
				"ids, what publishing would refuse or warn about, and the step kinds its trigger may use with " +
				"their settings and paths.",
			Schema: schema(`{"type":"object","required":["workflow_id"],"properties":{` +
				`"workflow_id":{"type":"string","description":"Workflow id (wf_...)."}}}`),
		}, handle: e.getWorkflow, was: map[string]string{"workflow": "workflow_id", "id": "workflow_id"}},
		{spec: contracts.ToolSpec{
			Name: "get_workflow_reference",
			Description: "Read how workflows are built: every trigger with the steps it may use, every step " +
				"kind with its settings and paths, and the fields a condition may read. Read it before drafting " +
				"a new workflow.",
			Schema: schema(`{"type":"object","properties":{"trigger":{"type":"string","description":"A ` +
				`trigger key, such as conversation.first_message, to read only what it may use; leave out for ` +
				`every trigger."}}}`),
		}, handle: e.getWorkflowReference},
		{spec: contracts.ToolSpec{
			Name: "explain_workflow_run",
			Description: "Explain why workflows did what they did on a conversation: each live workflow's " +
				"verdict on each event (started, or skipped and why), and each run's steps with the path it took.",
			Schema: schema(`{"type":"object","properties":{` +
				`"conversation_number":{"type":"integer","description":"Conversation number, such as 123 for ` +
				`#123; leave out for this thread's."},` +
				`"run_id":{"type":"string","description":"A workflow run id (wfr_...) instead of a conversation."},` +
				`"workflow_id":{"type":"string","description":"Only this workflow's runs (wf_...)."}}}`),
		}, handle: e.explainWorkflowRun, was: map[string]string{"conversation": "conversation_number"}},
	}
}

/*
listWorkflows lists the workflows in priority order.
*/
func (e *Engine) listWorkflows(ctx context.Context, s *session, _ json.RawMessage) (result, error) {
	list, err := e.Services.Workflows.List(ctx, s.m, false)
	if err != nil {
		return result{}, err
	}
	var b strings.Builder
	for _, wf := range list {
		changed := ""
		if wf.LiveHash == nil || *wf.LiveHash != wf.DraftHash {
			changed = " · working copy not published"
		}
		fmt.Fprintf(&b, "- [%s](%s) · workflow_id %s · trigger %s · %s · %s%s\n", wf.Name,
			e.link(s, linkWorkflow, models.PrefixWorkflow, wf.ID), models.PublicID(models.PrefixWorkflow, wf.ID),
			wf.Trigger, wf.Status, wf.Kind, changed)
	}
	if len(list) == 0 {
		b.WriteString("This workspace has no workflows yet.")
	}
	return result{text: b.String(), step: Step{
		Tool: "list_workflows", Summary: fmt.Sprintf("Listed %d workflows", len(list)),
	}}, nil
}

/*
getWorkflow reads one workflow with its outline, report and reference.
*/
func (e *Engine) getWorkflow(ctx context.Context, s *session, args json.RawMessage) (result, error) {
	id, err := e.idArg(args, "workflow_id", models.PrefixWorkflow)
	if err != nil {
		return result{}, err
	}
	wf, err := e.Services.Workflows.Show(ctx, s.m, id)
	if err != nil {
		return result{}, err
	}
	text, err := e.workflowText(ctx, s, wf)
	if err != nil {
		return result{}, err
	}
	return result{text: text, step: Step{Tool: "get_workflow_by_id", Summary: "Read the workflow " + wf.Name}}, nil
}

/*
getWorkflowReference reads how workflows are built.
*/
func (e *Engine) getWorkflowReference(_ context.Context, _ *session, args json.RawMessage) (result, error) {
	var in struct {
		Trigger string `json:"trigger"`
	}
	if err := decode(args, &in); err != nil {
		return result{}, err
	}
	return result{text: workflowReference(strings.TrimSpace(in.Trigger)), step: Step{
		Tool: "get_workflow_reference", Summary: "Read how workflows are built",
	}}, nil
}

/*
workflowText is everything about one workflow the model edits by: who
it is, its link, the working copy as an outline, what publishing would
find, and the reference of its trigger. A suggestion open on it shows
its working copy as suggested instead.
*/
func (e *Engine) workflowText(ctx context.Context, s *session, wf models.Workflow) (string, error) {
	spec, err := workflowspec.Parse(wf.Draft)
	if err != nil {
		return "", err
	}
	if suggested, ok := s.suggestedWorkflow(wf.ID); ok {
		spec = suggested
	}
	report, err := e.Services.Workflows.CheckDraft(ctx, s.m, wf, spec)
	if err != nil {
		return "", err
	}
	public := models.PublicID(models.PrefixWorkflow, wf.ID)
	var b strings.Builder
	fmt.Fprintf(&b, "Workflow %q\nworkflow_id: %s\nStatus: %s\nKind: %s\nLink: %s\n", wf.Name, public, wf.Status,
		wf.Kind, e.link(s, linkWorkflow, models.PrefixWorkflow, wf.ID))
	if wf.Description != nil {
		fmt.Fprintf(&b, "Description: %s\n", *wf.Description)
	}
	if wf.LiveHash == nil || *wf.LiveHash != wf.DraftHash {
		b.WriteString("The working copy differs from what runs.\n")
	}
	b.WriteString("\nWorking copy:\n" + workflowOutline(spec))
	b.WriteString("\n" + reportText(report))
	fmt.Fprintf(&b, "\nTo change it, call propose_workflow_edit with workflow_id %s.\n\n", public)
	b.WriteString(workflowReference(spec.Trigger))
	return truncate(b.String(), 40000), nil
}

/*
reportText writes what publishing would refuse and warn about.
*/
func reportText(r workflowspec.Report) string {
	if len(r.Errors) == 0 && len(r.Warnings) == 0 {
		return "Check: nothing stops it from being published.\n"
	}
	var b strings.Builder
	if len(r.Errors) > 0 {
		b.WriteString("Check, errors that stop it from being published:\n" + issueLines(r.Errors))
	}
	if len(r.Warnings) > 0 {
		b.WriteString("Check, warnings a teammate must accept when publishing:\n" + issueLines(r.Warnings))
	}
	return b.String()
}

/*
issueLines writes issues one per line, with what each is about.
*/
func issueLines(list []workflowspec.Issue) string {
	var b strings.Builder
	for _, i := range list {
		fmt.Fprintf(&b, "- %s: %s", i.Code, i.Message)
		var where []string
		for _, at := range [][2]string{{"step", i.StepID}, {"path", i.PathID}, {"setting", i.Field}} {
			if at[1] != "" {
				where = append(where, at[0]+" "+at[1])
			}
		}
		if len(where) > 0 {
			fmt.Fprintf(&b, " (%s)", strings.Join(where, ", "))
		}
		b.WriteString("\n")
	}
	return b.String()
}

/*
explainWorkflowRun explains what workflows did on a conversation.
*/
func (e *Engine) explainWorkflowRun(ctx context.Context, s *session, args json.RawMessage) (result, error) {
	var in runArgs
	if err := decode(args, &in); err != nil {
		return result{}, err
	}
	if !s.m.Can("workflows:read") {
		return result{}, fmt.Errorf("%w: reading workflow runs needs workflows:read", errBadArguments)
	}
	var only *ids.UUID
	if strings.TrimSpace(in.Workflow) != "" {
		id, err := e.publicID(in.Workflow, models.PrefixWorkflow)
		if err != nil {
			return result{}, err
		}
		only = &id
	}
	names := e.workflowNames(ctx, s)
	var text string
	err := e.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		c, err := e.runConversation(ctx, q, s, in)
		if err != nil {
			return err
		}
		text, err = e.automationOf(ctx, q, s, explained{conversation: c, only: only, names: names})
		return err
	})
	if errors.Is(err, database.ErrNotFound) {
		return result{}, fmt.Errorf("%w: no run with this id", errBadArguments)
	}
	if err != nil {
		return result{}, err
	}
	s.tainted = true
	return result{text: truncate(text, 30000), step: Step{
		Tool: "explain_workflow_run", Summary: "Read what workflows did on a conversation",
	}}, nil
}

/*
runConversation finds the conversation a run explanation is about: the
run's, or the one named by number, or this thread's, among those the
teammate can see.
*/
func (e *Engine) runConversation(ctx context.Context, q *models.Queries, s *session, in runArgs,
) (models.Conversation, error) {
	if strings.TrimSpace(in.Run) == "" {
		r, err := ref(s, in.Number)
		if err != nil {
			return models.Conversation{}, err
		}
		return conversations.Find(ctx, q, s.m, r)
	}
	id, err := e.publicID(in.Run, models.PrefixWorkflowRun)
	if err != nil {
		return models.Conversation{}, err
	}
	row, err := q.FindWorkflowRun(ctx, models.FindWorkflowRunParams{WorkspaceID: s.m.Workspace.ID, ID: id})
	if err != nil {
		return models.Conversation{}, err
	}
	if row.ConversationID == nil {
		return models.Conversation{}, fmt.Errorf("%w: this run has no conversation", errBadArguments)
	}
	return conversations.Find(ctx, q, s.m, conversations.Ref{ID: *row.ConversationID})
}

/*
runArgs are what explain_workflow_run reads: a conversation by number,
or a run by id, and the one workflow to show runs of.
*/
type runArgs struct {
	Number   int64  `json:"conversation_number"`
	Run      string `json:"run_id"`
	Workflow string `json:"workflow_id"`
}

/*
explained is what one explanation covers: the conversation, the one
workflow whose runs it shows, if any, and every workflow's name.
*/
type explained struct {
	conversation models.Conversation
	only         *ids.UUID
	names        map[string]string
}

/*
automationOf writes a conversation's match verdicts and runs, newest
first.
*/
func (e *Engine) automationOf(ctx context.Context, q *models.Queries, s *session, x explained) (string, error) {
	c, only, names := x.conversation, x.only, x.names
	ws := s.m.Workspace.ID
	logs, err := q.ListWorkflowMatchLogs(ctx, models.ListWorkflowMatchLogsParams{
		WorkspaceID: ws, ConversationID: c.ID, RowLimit: matchLogsRead,
	})
	if err != nil {
		return "", fmt.Errorf("list match logs: %w", err)
	}
	runs, err := q.ListConversationWorkflowRuns(ctx, models.ListConversationWorkflowRunsParams{
		WorkspaceID: ws, ConversationID: &c.ID, RowLimit: runsRead,
	})
	if err != nil {
		return "", fmt.Errorf("list runs: %w", err)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Conversation [#%d](%s)\n\nWhat each live workflow decided on each event, newest first:\n",
		c.Number, e.conversationLink(s, c.ID))
	if len(logs) == 0 {
		b.WriteString("- No events were matched against workflows, or the record of them has expired.\n")
	}
	for _, l := range logs {
		writeVerdicts(&b, l, names)
	}
	b.WriteString("\nRuns, newest first:\n")
	shown := 0
	for _, r := range runs {
		if only != nil && r.WorkflowID != *only {
			continue
		}
		shown++
		steps, err := q.ListWorkflowRunSteps(ctx, models.ListWorkflowRunStepsParams{WorkspaceID: ws, RunID: r.ID})
		if err != nil {
			return "", fmt.Errorf("list run steps: %w", err)
		}
		writeRun(&b, r, steps)
	}
	if shown == 0 {
		b.WriteString("- No workflow ran on this conversation.\n")
	}
	return b.String(), nil
}

/*
writeVerdicts writes one event's verdicts.
*/
func writeVerdicts(b *strings.Builder, l models.WorkflowMatchLog, names map[string]string) {
	var verdicts []struct {
		WorkflowID string `json:"workflow_id"`
		VersionID  string `json:"version_id"`
		Verdict    string `json:"verdict"`
		Reason     string `json:"reason"`
		FailedRule string `json:"failed_rule"`
	}
	_ = json.Unmarshal(l.Verdicts, &verdicts)
	fmt.Fprintf(b, "- %s, event %s:\n", l.CreatedAt.UTC().Format("2006-01-02 15:04:05"), l.Trigger)
	if len(verdicts) == 0 {
		b.WriteString("    no live workflow runs on this trigger\n")
	}
	for _, v := range verdicts {
		name := names[v.WorkflowID]
		if name == "" {
			name = "an archived workflow"
		}
		fmt.Fprintf(b, "    %q (%s): %s", name, v.WorkflowID, v.Verdict)
		if v.Reason != "" {
			fmt.Fprintf(b, ", reason %s", v.Reason)
		}
		if v.FailedRule != "" {
			fmt.Fprintf(b, ", failed rule %s", v.FailedRule)
		}
		b.WriteString("\n")
	}
}

/*
writeRun writes one run with the steps it took.
*/
func writeRun(b *strings.Builder, r models.ListConversationWorkflowRunsRow, steps []models.WorkflowRunStep) {
	fmt.Fprintf(b, "- run %s of %q (%s) · started %s on %s · %s", models.PublicID(models.PrefixWorkflowRun, r.ID),
		r.WorkflowName, models.PublicID(models.PrefixWorkflow, r.WorkflowID),
		r.StartedAt.UTC().Format("2006-01-02 15:04:05"), r.Trigger, r.Status)
	if r.EndReason != nil {
		fmt.Fprintf(b, " · ended: %s", *r.EndReason)
	}
	if r.VersionID != nil {
		fmt.Fprintf(b, " · version %s", models.PublicID(models.PrefixWorkflowVersion, *r.VersionID))
	}
	if len(r.Error) > 0 && string(r.Error) != "null" {
		fmt.Fprintf(b, " · error %s", truncate(string(r.Error), 500))
	}
	b.WriteString("\n")
	for _, st := range steps {
		fmt.Fprintf(b, "    step %s · %s · %s", st.StepID, st.Kind, st.Status)
		if st.Outcome != nil {
			fmt.Fprintf(b, " · outcome %s", *st.Outcome)
		}
		if st.PathID != nil {
			fmt.Fprintf(b, " · took path %s", *st.PathID)
		}
		if len(st.Detail) > 0 && string(st.Detail) != "null" && string(st.Detail) != "{}" {
			fmt.Fprintf(b, " · %s", truncate(string(st.Detail), 400))
		}
		b.WriteString("\n")
	}
}

/*
workflowNames names every workflow by its public id, archived ones too.
*/
func (e *Engine) workflowNames(ctx context.Context, s *session) map[string]string {
	out := map[string]string{}
	for _, archived := range []bool{false, true} {
		list, err := e.Services.Workflows.List(ctx, s.m, archived)
		if err != nil {
			continue
		}
		for _, wf := range list {
			out[models.PublicID(models.PrefixWorkflow, wf.ID)] = wf.Name
		}
	}
	return out
}
