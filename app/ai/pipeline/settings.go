/*
|--------------------------------------------------------------------------
| Turn Settings
|--------------------------------------------------------------------------
|
| Everything one agent turn may be told to do differently. Today a turn
| takes them from the agent's settings; a workflow step that lets the
| agent handle a conversation passes its own: its mode, which tools it
| may use, the procedure it must follow, the topics that always go to a
| person, where a handoff goes, whether to ask the customer to confirm
| resolution, and how many replies it may send. When a workflow
| controls a conversation's AI and no agent step is active, the gate
| keeps the agent quiet.
|
| The rules are not only words in the prompt: after the turn, code
| hands the conversation off itself when the mode or a tag demands it
| and the agent did not.
|
*/

package pipeline

import (
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/ai"
	"github.com/dimmihq/dimmi/app/enums"
)

/*
The turn modes: full answers from the knowledge, triage only sorts the
conversation and hands it to a person.
*/
const (
	ModeFull       = string(enums.AIHandoffModeFull)
	ModeTriageOnly = string(enums.AIHandoffModeTriageOnly)
)

/*
TurnSettings shape one turn. Mode is full or triage only; Tools lists
the tools the agent may call, by name, where ToolActions stands for
every action the workspace published; Instructions is a mandatory
procedure that follows the workspace guidance; AlwaysHandoffTags are
the tags whose presence makes a handoff mandatory; HandoffTeamID and
RaisePriority route a handoff; ConfirmResolution asks the customer
whether the answer solved it; MaxReplies caps the agent's replies in
the conversation; Workflow says whether a workflow controls the
conversation's AI.
*/
type TurnSettings struct {
	Mode              string
	Tools             []string
	Instructions      string
	AlwaysHandoffTags []ids.UUID
	HandoffTeamID     *ids.UUID
	RaisePriority     bool
	ConfirmResolution bool
	MaxReplies        int32
	Workflow          WorkflowControl
}

/*
WorkflowControl is the seam a workflow engine fills: Controlled when a
workflow owns the conversation's AI, StepActive while one of its steps
lets the agent handle the conversation.
*/
type WorkflowControl struct {
	Controlled bool
	StepActive bool
}

/*
settingsFor builds a turn's settings from the agent's own: its mode and
the tools it allows, its reply limit, and its handoff routing and rules.
*/
func settingsFor(e *env) *TurnSettings {
	h := ai.ReadHandoff(e.Agent.Handoff)

	s := &TurnSettings{
		Mode: ModeFull, Tools: []string{toolSearchKnowledge, toolHandoff, toolTriage, toolProcedure, ToolActions},
		AlwaysHandoffTags: h.AlwaysTags, HandoffTeamID: h.TargetTeamID, RaisePriority: h.RaisePriority,
		MaxReplies: e.Agent.MaxAiReplies,
		Workflow:   WorkflowControl{Controlled: e.Conversation.AiMode == "workflow"},
	}
	if h.Mode == enums.AIHandoffModeTriageOnly {
		s.Mode, s.Tools = ModeTriageOnly, []string{toolHandoff, toolTriage}
	}

	applyWorkflowStep(e, s)
	return s
}

/*
ToolActions is the entry of TurnSettings.Tools that allows every action
the workspace published; an action's own name allows that one alone.
*/
const ToolActions = "actions"

/*
allows reports whether the turn may call the tool.
*/
func (s *TurnSettings) allows(name string, action bool) bool {
	return slices.Contains(s.Tools, name) || action && slices.Contains(s.Tools, ToolActions)
}

/*
rulesText is the {{rules}} block of the answer prompt.
*/
func rulesText(e *env, s *TurnSettings) string {
	var lines []string
	if s.Mode == ModeTriageOnly {
		lines = append(lines, "Mode: triage only. Triage the conversation and hand it to a teammate; "+
			"do not answer product questions.")
	}
	var names []string
	for _, id := range s.AlwaysHandoffTags {
		names = append(names, tagName(e.Triage, id))
	}
	if len(names) > 0 {
		lines = append(lines, "Always hand off when the conversation is about: "+strings.Join(names, ", ")+".")
	}
	if text := strings.TrimSpace(s.Instructions); text != "" {
		lines = append(lines, text)
	}
	if len(lines) == 0 {
		return "No extra rules."
	}
	return strings.Join(lines, "\n")
}

/*
enforceRules hands the conversation off when the turn's rules demand
it and the agent did not: triage-only mode, or a tag that always goes
to a person on the conversation as the turn leaves it. It reports
whether it did.
*/
func enforceRules(st *run) bool {
	if st.handoff != nil {
		return false
	}
	if st.settings.Mode != ModeTriageOnly && !alwaysTagged(st) {
		return false
	}
	st.handoff = &handoffRequest{reason: "workspace_rule"}
	st.flags = append(st.flags, "enforced_handoff")
	return true
}

/*
alwaysTagged reports whether the conversation carries a tag that always
goes to a person, counting the triage the turn asked for.
*/
func alwaysTagged(st *run) bool {
	for _, id := range st.settings.AlwaysHandoffTags {
		_, on := st.env.Triage.tagAuthors[id]
		added := slices.Contains(st.changes.addTags, id)
		removed := slices.Contains(st.changes.removeTags, id)
		if (on || added) && !removed {
			return true
		}
	}
	return false
}
