/*
|--------------------------------------------------------------------------
| Eligibility Gate
|--------------------------------------------------------------------------
|
| Before any model is called, the gate answers whether the agent may
| speak here at all: the agent is on, the channel is allowed, the
| conversation is live and nobody on the team has taken it, and the
| budgets hold. It decides from what the run loaded and spends no
| tokens.
|
| A customer already talking to the agent is never left hanging: when
| the schedule, the spend cap, a missing model or the reply limit stops
| the agent mid-conversation, the conversation is handed to the team
| with a line saying so. A customer the agent never served simply
| waits for a person.
|
*/

package pipeline

import (
	"slices"

	"github.com/dimmihq/dimmi/app/enums"
)

/*
gate returns the first rule that stops the agent, or nil. It decides
on policy alone and understands no language.
*/
func (r *Runner) gate(e *env, s *TurnSettings) *decision {
	if reason := eligibility(e); reason != "" {
		return skipped(reason)
	}
	if s.Workflow.Controlled && !s.Workflow.StepActive {
		return skipped("workflow_controls_ai")
	}
	if reason := humanGate(e); reason != "" {
		return skipped(reason)
	}
	if len(e.Pending) == 0 || e.pendingText() == "" {
		return skipped("no_text")
	}
	if reason := pauseGate(e); reason != "" {
		if e.Conversation.AiState != aiStateActive {
			return skipped(reason)
		}
		return handoff(reason, "", e.handoffKey())
	}
	if s.MaxReplies > 0 && e.Conversation.AiReplyCount >= s.MaxReplies {
		return handoff("max_replies", "", "max_replies")
	}
	return nil
}

/*
eligibility refuses conversations the agent must never touch.
*/
func eligibility(e *env) string {
	c := e.Conversation
	switch {
	case c.DeletedAt != nil || c.Spam:
		return "conversation_ineligible"
	case e.Contact != nil && (e.Contact.BlockedAt != nil || e.Contact.DeletedAt != nil):
		return "conversation_ineligible"
	case !e.Agent.Enabled:
		return "agent_disabled"
	case len(e.Agent.EnabledChannels) > 0 && !slices.Contains(e.Agent.EnabledChannels, e.ChannelType):
		return "agent_disabled"
	case slices.Contains(e.Agent.ExcludedChannelConnectionIds, c.ChannelConnectionID):
		return "agent_disabled"
	case c.AiState == aiStateHandedOff:
		return "handed_off"
	case e.Workspace.SuspendedAt != nil:
		return "workspace_ineligible"
	}
	return ""
}

/*
humanGate steps back when a teammate already answered the trigger or
is working the conversation: never when assigned, while the assignee
is around by default, and only for the reply itself when the agent
always answers.
*/
func humanGate(e *env) string {
	if e.ReplyAfter == string(enums.AuthorTypeMember) {
		return "human_replied"
	}
	if e.Conversation.AssigneeMemberID == nil {
		return ""
	}
	switch e.Agent.AssignedBehavior {
	case "always":
		return ""
	case "never_when_assigned":
		return "assigned"
	}
	if e.Facts.assigneeActive {
		return "human_active"
	}
	return ""
}

/*
pauseGate names the rule that pauses the agent: its schedule, the
spend cap, or no usable answer model.
*/
func pauseGate(e *env) string {
	if reason := scheduleGate(e); reason != "" {
		return reason
	}
	if e.Facts.spendCapped {
		return "spend_cap"
	}
	if e.Agent.AnswerModel == nil || *e.Agent.AnswerModel == "" || e.AnswerKey == nil {
		return "provider_unavailable"
	}
	if s := e.AnswerKey.Status; s == "invalid" || s == "error" {
		return "provider_unavailable"
	}
	return ""
}

/*
scheduleGate applies the schedule setting. Office hours that cannot be
read count as closed for both schedules, so a broken setting never
lets the agent answer when it should not.
*/
func scheduleGate(e *env) string {
	switch e.Agent.Schedule {
	case string(enums.AIScheduleOutsideOffice):
		if e.Facts.hoursErr != nil || e.Facts.withinHours {
			return "schedule"
		}
	case string(enums.AIScheduleInsideOffice):
		if e.Facts.hoursErr != nil || !e.Facts.withinHours {
			return "schedule"
		}
	}
	return ""
}

/*
availability is what the answer prompt is told about handing off:
available, later when the team is offline, or unavailable when the
agent is set to say it does not know without offering a person.
*/
func (e *env) availability() string {
	switch e.Agent.UnknownBehavior {
	case "offer_handoff", "handoff":
	default:
		return "unavailable"
	}
	if e.Facts.onlineMembers == 0 {
		return "later"
	}
	if e.Facts.hoursErr == nil && !e.Facts.withinHours {
		return "later"
	}
	return "available"
}

/*
handoffKey names the default line a handoff without model text uses.
*/
func (e *env) handoffKey() string {
	switch e.availability() {
	case "later":
		return "handoff_later"
	case "unavailable":
		return "handoff_unavailable"
	}
	return "handoff"
}
