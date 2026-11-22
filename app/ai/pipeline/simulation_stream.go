/*
|--------------------------------------------------------------------------
| A Simulated Turn, As It Happens
|--------------------------------------------------------------------------
|
| A teammate previewing a procedure watches the agent work: each tool it
| finishes is sent to them at once, on their own channel, under the id
| their preview gave the turn. The procedure it follows, each step it
| finishes or still waits on, every action it calls or is refused, a
| search, a handover. Nothing is kept for replay, and a broadcast that
| fails changes nothing: the request answers with the whole turn anyway.
|
*/

package pipeline

import (
	"context"
	"encoding/json"
	"strings"

	fbroadcasting "github.com/dimmihq/dimmi/framework/broadcasting"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/ai/toolset"
	appbroadcasting "github.com/dimmihq/dimmi/app/broadcasting"
)

/*
EventPreviewStep is the event a simulated turn sends for each tool it
finished.
*/
const EventPreviewStep = "procedure.preview.step"

/*
PreviewStep is one thing the agent did in a simulated turn: the tool,
what it did in words, and whether it went through. For an action, the
tool is the action's name and Action says so, Refused when a guardrail
held it back; the preview names it by its title.
*/
type PreviewStep struct {
	Tool    string `json:"tool"`
	Summary string `json:"summary"`
	OK      bool   `json:"ok"`
	Action  bool   `json:"action,omitempty"`
	Refused bool   `json:"refused,omitempty"`
}

/*
finished is a tool call a simulated turn just ran.
*/
type finished struct {
	tool    string
	args    json.RawMessage
	result  string
	failed  bool
	refused bool
}

/*
publishPreview sends a finished tool call to the teammate previewing.
*/
func (r *Runner) publishPreview(ctx context.Context, st *run, f finished) {
	if st.sim == nil || st.sim.in.StreamID == "" || r.Publisher == nil {
		return
	}
	step, ok := r.previewStep(st, f)
	if !ok {
		return
	}
	r.publishStep(ctx, st, step)
}

/*
EventSimulationRun is the event of a saved simulation's run: a turn
started or finished, the checks being judged, or its verdict.
*/
const EventSimulationRun = "procedure.simulation.run"

/*
PublishRun sends an event of a saved simulation's run to the teammate
who started it. Nothing is stored.
*/
func (r *Runner) PublishRun(ctx context.Context, member ids.UUID, data map[string]any) {
	if r.Publisher == nil {
		return
	}
	_ = r.Publisher.Publish(ctx, []string{appbroadcasting.MemberChannel(member)}, fbroadcasting.Message{
		Event: EventSimulationRun, Ephemeral: true, Data: data,
	})
}

/*
publishStep sends one step of the preview's log to the teammate
previewing.
*/
func (r *Runner) publishStep(ctx context.Context, st *run, step PreviewStep) {
	if st.sim == nil || st.sim.in.StreamID == "" || r.Publisher == nil {
		return
	}
	_ = r.Publisher.Publish(ctx, []string{appbroadcasting.MemberChannel(st.sim.in.MemberID)}, fbroadcasting.Message{
		Event: EventPreviewStep, Ephemeral: true,
		Data: map[string]any{"stream_id": st.sim.in.StreamID, "step": step},
	})
}

/*
previewStep reads a finished tool call as a step of the preview's log.
Loading tools is the agent's own bookkeeping and is left out.
*/
func (r *Runner) previewStep(st *run, f finished) (PreviewStep, bool) {
	if t, known := r.registry(st)[f.tool]; known && t.action {
		return PreviewStep{Tool: f.tool, OK: !f.failed && !f.refused, Action: true, Refused: f.refused}, true
	}
	said := map[string]string{
		toolSearchKnowledge: "Searched the help center",
		toolHandoff:         "Handed the conversation to the team",
		toolTriage:          triageSummary(f),
		toolOpenTicket:      "Opened a ticket",
	}
	switch f.tool {
	case toolset.LoadTool:
		return PreviewStep{}, false
	case toolProcedure:
		return PreviewStep{Tool: f.tool, Summary: followedSummary(st), OK: !f.failed && st.proc != nil}, true
	case toolProcedureStatus:
		summary := statusSummary(f)
		return PreviewStep{Tool: f.tool, Summary: summary, OK: !f.failed}, summary != ""
	}
	summary, ok := said[f.tool]
	if !ok {
		summary = "Ran " + f.tool
	}
	return PreviewStep{Tool: f.tool, Summary: summary, OK: !f.failed}, true
}

/*
triageSummary says how the agent labeled the conversation for the
team's inbox: the title, tags, team or priority it set. Nobody is told
and the customer never sees it.
*/
func triageSummary(f finished) string {
	var in map[string]json.RawMessage
	_ = json.Unmarshal(f.args, &in)
	var set []string
	for _, field := range []string{"title", "tags", "team", "priority"} {
		if raw, ok := in[field]; ok && string(raw) != "null" {
			set = append(set, field)
		}
	}
	if len(set) == 0 {
		return "Labeled the conversation for the inbox"
	}
	last := len(set) - 1
	if last == 0 {
		return "Set the conversation's " + set[0] + " for the inbox"
	}
	return "Set the conversation's " + strings.Join(set[:last], ", ") + " and " + set[last] + " for the inbox"
}

/*
followedSummary says which procedure the agent began to follow.
*/
func followedSummary(st *run) string {
	if st.proc == nil {
		return "Found no procedure to follow"
	}
	return "Followed " + st.proc.live.row.Name
}

/*
refusedSummary says what a refused report means for the person
watching: the step still waits on the customer. A refused end of the
procedure says nothing; the procedure simply goes on.
*/
func refusedSummary(status, step, result string) string {
	if !strings.HasPrefix(status, "step_") || strings.Contains(result, `"undecided":true`) {
		return ""
	}
	if strings.Contains(result, "is not finished") {
		return "Step " + step + " still needs more from the customer"
	}
	return "Step " + step + " waits for the customer's reply"
}

/*
statusSummary says what the agent reported about the procedure: a step
finished, a step it must still do with the customer, or its end.
*/
func statusSummary(f finished) string {
	type report struct {
		Status string `json:"status"`
		Step   string `json:"step"`
	}
	var in struct {
		report
		Arguments *report `json:"arguments"`
	}
	_ = json.Unmarshal(f.args, &in)
	if in.Arguments != nil {
		in.report = *in.Arguments
	}
	if strings.Contains(f.result, `"ok":false`) {
		return refusedSummary(in.Status, in.Step, f.result)
	}
	if strings.Contains(f.result, `"nothing_changed":true`) {
		return ""
	}
	switch in.Status {
	case "step_done":
		return "Finished step " + in.Step
	case "step_skipped":
		if strings.Contains(f.result, `"answered":true`) {
			return "Finished step " + in.Step
		}
		return "Skipped step " + in.Step + ": the customer already covered it"
	case "completed":
		return "Finished the procedure"
	case "exited_topic_changed":
		return "Left the procedure: the customer moved on"
	}
	return "Updated the procedure"
}
