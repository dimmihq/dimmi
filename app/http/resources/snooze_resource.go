/*
|--------------------------------------------------------------------------
| Snooze Resources
|--------------------------------------------------------------------------
|
| The snooze lists set, and the choices of one conversation each with
| the moment it ends for the reader.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/models"
)

/*
SnoozeLists are the workspace's list, null when it uses the built-in
presets, and each team's by team id.
*/
type SnoozeLists struct {
	Object    string                                  `json:"object"`
	Workspace []conversations.SnoozeOption            `json:"workspace"`
	Teams     map[string][]conversations.SnoozeOption `json:"teams"`
	Defaults  []conversations.SnoozeOption            `json:"defaults"`
}

/*
NewSnoozeLists shapes the lists.
*/
func NewSnoozeLists(l conversations.SnoozeLists) SnoozeLists {
	out := SnoozeLists{
		Object: "snooze_lists", Workspace: l.Workspace, Teams: map[string][]conversations.SnoozeOption{},
		Defaults: conversations.BuiltInSnoozeOptions(),
	}

	for team, options := range l.Teams {
		out.Teams[models.PublicID(models.PrefixTeam, team)] = options
	}

	return out
}

/*
SnoozeChoice is one choice of a conversation and when it ends.
*/
type SnoozeChoice struct {
	conversations.SnoozeOption

	Until time.Time `json:"until"`
}

/*
NewSnoozeChoices shapes the choices.
*/
func NewSnoozeChoices(options []conversations.ResolvedOption) []SnoozeChoice {
	out := make([]SnoozeChoice, len(options))
	for i, o := range options {
		out[i] = SnoozeChoice{SnoozeOption: o.SnoozeOption, Until: o.Until}
	}

	return out
}
