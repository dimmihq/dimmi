/*
|--------------------------------------------------------------------------
| Triggers
|--------------------------------------------------------------------------
|
| A trigger is what starts a workflow. Each one belongs to a group that
| decides which steps it may use: workflows that start a conversation
| may talk to the customer and hand it to dimmi; workflows that react to
| the team may only act in the background. A trigger also fixes how
| often it may run again on one conversation and which channels it can
| run on.
|
*/

package workflowspec

import "slices"

/*
The trigger groups.
*/
const (
	GroupStart    = "start"
	GroupDuring   = "during"
	GroupTeam     = "team"
	GroupDimmi    = "dimmi"
	GroupSLA      = "sla"
	GroupReusable = "reuse"
)

/*
The trigger keys.
*/
const (
	TriggerOpener            = "conversation.opener"
	TriggerFirstMessage      = "conversation.first_message"
	TriggerCustomerReopened  = "conversation.customer_reopened"
	TriggerCustomerMessage   = "message.customer"
	TriggerCustomerQuiet     = "conversation.customer_quiet"
	TriggerRated             = "conversation.rated"
	TriggerTeammateMessage   = "message.teammate"
	TriggerNoteCreated       = "note.created"
	TriggerAssignmentChanged = "conversation.assignment_changed"
	TriggerStatusChanged     = "conversation.status_changed"
	TriggerTeammateQuiet     = "conversation.teammate_quiet"
	TriggerDimmiHandedOver   = "dimmi.handed_over"
	TriggerDimmiResolved     = "dimmi.resolved"
	TriggerSLAWarning        = "sla.warning"
	TriggerSLABreached       = "sla.breached"
	TriggerReusable          = "reusable"
)

/*
The re-entry modes.
*/
const (
	ReentryEveryTime = "every_time"
	ReentryOnce      = "once_per_conversation"
	ReentryCooldown  = "cooldown"
)

/*
Trigger describes one trigger: its group, the re-entry modes it allows
and its default, whether it may run on any channel or only some, and
which customer-facing steps it may use.
*/
type Trigger struct {
	Key                string
	Name               string
	Group              string
	Reentry            []string
	Default            string
	Channels           []string
	Talks              []string
	Dimmi              bool
	ConversationFields bool
}

/*
customerSteps are the steps that speak to the customer.
*/
func customerSteps() []string {
	return []string{KindMessage, KindButtons, KindCollect, KindReplyTime}
}

/*
Triggers returns every trigger in list order.
*/
func Triggers() []Trigger {
	return slices.Concat(startTriggers(), duringTriggers(), teamTriggers(), otherTriggers())
}

/*
reentryOnce and reentryAny are the re-entry modes a trigger may allow.
*/
func reentryOnce() []string { return []string{ReentryOnce} }

func reentryAny() []string { return []string{ReentryEveryTime, ReentryOnce, ReentryCooldown} }

/*
startTriggers returns the triggers that start a conversation.
*/
func startTriggers() []Trigger {
	return []Trigger{
		{
			Key: TriggerOpener, Name: "Customer starts a new conversation", Group: GroupStart, Reentry: reentryOnce(),
			Default: ReentryOnce, Channels: []string{"widget"}, Talks: customerSteps(), Dimmi: true,
		},
		{
			Key: TriggerFirstMessage, Name: "Customer sends their first message", Group: GroupStart,
			Reentry: reentryOnce(),
			Default: ReentryOnce, Talks: customerSteps(), Dimmi: true, ConversationFields: true,
		},
		{
			Key: TriggerCustomerReopened, Name: "Customer writes on a closed conversation", Group: GroupStart,
			Reentry: reentryAny(), Default: ReentryEveryTime, Talks: customerSteps(), Dimmi: true,
			ConversationFields: true,
		},
	}
}

/*
duringTriggers returns the triggers of a conversation under way.
*/
func duringTriggers() []Trigger {
	return []Trigger{
		{
			Key: TriggerCustomerMessage, Name: "Customer sends a message", Group: GroupDuring, Reentry: reentryAny(),
			Default: ReentryEveryTime, ConversationFields: true,
		},
		{
			Key: TriggerCustomerQuiet, Name: "Customer stops replying", Group: GroupDuring, Reentry: reentryAny(),
			Default: ReentryEveryTime, Talks: customerSteps(), Dimmi: true, ConversationFields: true,
		},
		{
			Key: TriggerRated, Name: "Customer rates the conversation", Group: GroupDuring, Reentry: reentryAny(),
			Default: ReentryOnce, Channels: []string{"widget", "email"}, Talks: []string{KindMessage},
			ConversationFields: true,
		},
	}
}

/*
teamTriggers returns the triggers of the team's work.
*/
func teamTriggers() []Trigger {
	return []Trigger{
		{
			Key: TriggerTeammateMessage, Name: "Teammate sends a reply", Group: GroupTeam, Reentry: reentryAny(),
			Default: ReentryEveryTime, ConversationFields: true,
		},
		{
			Key: TriggerNoteCreated, Name: "Teammate adds a note", Group: GroupTeam, Reentry: reentryAny(),
			Default: ReentryEveryTime, ConversationFields: true,
		},
		{
			Key: TriggerAssignmentChanged, Name: "Assignment changes", Group: GroupTeam, Reentry: reentryAny(),
			Default: ReentryEveryTime, ConversationFields: true,
		},
		{
			Key: TriggerStatusChanged, Name: "Conversation status changes", Group: GroupTeam, Reentry: reentryAny(),
			Default: ReentryEveryTime, ConversationFields: true,
		},
		{
			Key: TriggerTeammateQuiet, Name: "Customer waits for the team", Group: GroupTeam, Reentry: reentryOnce(),
			Default: ReentryOnce, Talks: []string{KindMessage, KindReplyTime}, ConversationFields: true,
		},
	}
}

/*
otherTriggers returns the dimmi, service level and reusable triggers.
*/
func otherTriggers() []Trigger {
	return []Trigger{
		{
			Key: TriggerDimmiHandedOver, Name: "dimmi hands over", Group: GroupDimmi, Reentry: reentryAny(),
			Default: ReentryEveryTime, ConversationFields: true,
		},
		{
			Key: TriggerDimmiResolved, Name: "dimmi resolves a conversation", Group: GroupDimmi, Reentry: reentryAny(),
			Default: ReentryEveryTime, Talks: []string{KindMessage}, ConversationFields: true,
		},
		{
			Key: TriggerSLAWarning, Name: "SLA about to breach", Group: GroupSLA, Reentry: reentryAny(),
			Default: ReentryEveryTime, ConversationFields: true,
		},
		{
			Key: TriggerSLABreached, Name: "SLA breached", Group: GroupSLA, Reentry: reentryAny(),
			Default: ReentryEveryTime, ConversationFields: true,
		},
		{
			Key: TriggerReusable, Name: "Run from other workflows", Group: GroupReusable, Reentry: reentryAny(),
			Default: ReentryEveryTime, Talks: customerSteps(), Dimmi: true, ConversationFields: true,
		},
	}
}

/*
FindTrigger returns the trigger with the key.
*/
func FindTrigger(key string) (Trigger, bool) {
	i := slices.IndexFunc(Triggers(), func(t Trigger) bool { return t.Key == key })
	if i < 0 {
		return Trigger{}, false
	}

	return Triggers()[i], true
}

/*
StartsConversation reports whether the trigger is one of the start
triggers, whose workflows always hold the conversation.
*/
func (t Trigger) StartsConversation() bool {
	return t.Group == GroupStart
}

/*
AllowsStep reports whether a step kind may appear under the trigger.
Actions and logic steps are allowed everywhere; customer-facing steps
only where the trigger may speak, and dimmi only where it may hand the
conversation to the agent.
*/
func (t Trigger) AllowsStep(kind string) bool {
	if kind == KindDimmi {
		return t.Dimmi
	}

	if slices.Contains(customerSteps(), kind) {
		return slices.Contains(t.Talks, kind)
	}

	return true
}
