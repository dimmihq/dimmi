/*
|--------------------------------------------------------------------------
| Notification Triggers
|--------------------------------------------------------------------------
|
| What a teammate can be told about, and how each one reaches them unless
| they changed it. Defaults live here, never in the database, so a member
| stores only what they changed. Email waits a little for the things a
| teammate often handles in the dashboard within minutes, and is skipped
| when they did; push waits a few seconds for a customer who sends
| several messages in a row, so a burst is one push, not five. A cell
| that is locked keeps an alert from being silenced by accident.
|
*/

package notifications

import (
	"slices"
	"time"
)

/*
The channels a teammate can be reached on.
*/
const (
	ChannelInApp = "in_app"
	ChannelEmail = "email"
	ChannelPush  = "push"
	ChannelSlack = "slack"
)

/*
Channels lists the channels in the order the settings show them. Slack
only reaches a member who linked their Slack user.
*/
func Channels() []string {
	return []string{ChannelInApp, ChannelEmail, ChannelPush, ChannelSlack}
}

/*
DestinationTriggers are the triggers a Slack channel can hear about.
*/
func DestinationTriggers() []string {
	return []string{
		TriggerNewConversation, TriggerCustomerReplied, TriggerAssigned, TriggerHandoff, TriggerSnoozeExpired,
	}
}

/*
The triggers teammates are told about.
*/
const (
	TriggerNewConversation  = "new_conversation"
	TriggerAssigned         = "conversation_assigned"
	TriggerCustomerReplied  = "customer_replied"
	TriggerMentioned        = "mentioned"
	TriggerHandoff          = "ai_handoff_requested"
	TriggerApprovalRequired = "ai_action_approval_requested"
	TriggerSnoozeExpired    = "snooze_expired"
	TriggerTeamJoined       = "team_joined"
)

/*
Trigger is one thing a teammate can be told about: its key, how the
settings name and explain it, its default per channel, how long email
and push wait before they go, and the channels that cannot be turned
off. Requires is a permission a member needs to ever receive it.
*/
type Trigger struct {
	Key         string
	Label       string
	Description string
	Defaults    map[string]bool
	EmailDelay  time.Duration
	PushDelay   time.Duration
	Locked      []string
	Requires    string
}

/*
Triggers returns every trigger in the order the settings show them.
*/
func Triggers() []Trigger {
	return []Trigger{
		{
			Key: TriggerNewConversation, Label: "New conversations",
			Description: "A customer starts a conversation you can see that nobody holds yet.",
			Defaults:    on(ChannelInApp, ChannelEmail, ChannelPush, ChannelSlack), EmailDelay: time.Minute,
		},
		{
			Key: TriggerAssigned, Label: "Assigned to you",
			Description: "Someone gives you a conversation.",
			Defaults:    on(ChannelInApp, ChannelEmail, ChannelPush, ChannelSlack), EmailDelay: time.Minute,
		},
		{
			Key: TriggerCustomerReplied, Label: "Customer replies",
			Description: "A customer writes again in a conversation you hold.",
			Defaults:    on(ChannelInApp, ChannelEmail, ChannelPush, ChannelSlack), EmailDelay: 3 * time.Minute,
			PushDelay: 10 * time.Second,
		},
		{
			Key: TriggerMentioned, Label: "Mentions",
			Description: "A teammate mentions you in a note.",
			Defaults:    on(ChannelInApp, ChannelEmail, ChannelPush, ChannelSlack), EmailDelay: time.Minute,
		},
		{
			Key: TriggerHandoff, Label: "Handed over by the agent",
			Description: "The dimmi agent hands a conversation to the team.",
			Defaults:    on(ChannelInApp, ChannelEmail, ChannelPush, ChannelSlack),
		},
		{
			Key: TriggerApprovalRequired, Label: "Actions waiting for approval",
			Description: "The agent asks to run an action that needs a teammate's yes.",
			Defaults:    on(ChannelInApp, ChannelEmail, ChannelPush), Requires: "ai_action_calls:approve",
		},
		{
			Key: TriggerSnoozeExpired, Label: "Snoozes ending",
			Description: "A conversation you hold wakes up from a snooze.",
			Defaults:    on(ChannelInApp, ChannelPush),
		},
		{
			Key: TriggerTeamJoined, Label: "Added to a team",
			Description: "Someone adds you to a team.",
			Defaults:    on(ChannelInApp, ChannelEmail),
		},
	}
}

/*
FindTrigger returns the trigger with the key.
*/
func FindTrigger(key string) (Trigger, bool) {
	for _, t := range Triggers() {
		if t.Key == key {
			return t, true
		}
	}
	return Trigger{}, false
}

/*
Default reports whether the channel is on for the trigger unless a
member changed it.
*/
func (t Trigger) Default(channel string) bool {
	return t.Defaults[channel]
}

/*
IsLocked reports whether the channel cannot be turned off.
*/
func (t Trigger) IsLocked(channel string) bool {
	return slices.Contains(t.Locked, channel)
}

/*
on returns a default row with the channels named on.
*/
func on(names ...string) map[string]bool {
	out := map[string]bool{}
	for _, n := range names {
		out[n] = true
	}
	return out
}
