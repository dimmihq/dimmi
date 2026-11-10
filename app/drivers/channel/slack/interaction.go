/*
|--------------------------------------------------------------------------
| Slack Interactions
|--------------------------------------------------------------------------
|
| A button press, a menu choice or a message shortcut, read from the
| payload Slack posts. Where it happened, who acted and what they
| pressed is all the product needs to find the conversation it is about.
|
*/

package slack

import (
	"encoding/json"
	"fmt"
	"strings"
)

/*
The interaction types the product reads.
*/
const (
	InteractionBlockActions  = "block_actions"
	InteractionMessageAction = "message_action"
	InteractionShortcut      = "shortcut"
	InteractionViewSubmit    = "view_submission"
)

/*
Interaction is one button press, menu choice or shortcut. Container and
Message say where it happened; ResponseURL answers the person who acted.
View is the form submitted, for a form submission.
*/
type Interaction struct {
	Type        string        `json:"type"`
	APIAppID    string        `json:"api_app_id"`
	CallbackID  string        `json:"callback_id"`
	TriggerID   string        `json:"trigger_id"`
	ResponseURL string        `json:"response_url"`
	MessageTS   string        `json:"message_ts"`
	Team        Ref           `json:"team"`
	User        InteractUser  `json:"user"`
	Channel     Ref           `json:"channel"`
	Container   Container     `json:"container"`
	Message     *Event        `json:"message"`
	Actions     []BlockAction `json:"actions"`
	View        *View         `json:"view"`
}

/*
Ref is a team or channel an interaction names.
*/
type Ref struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

/*
InteractUser is who acted.
*/
type InteractUser struct {
	ID     string `json:"id"`
	TeamID string `json:"team_id"`
	Name   string `json:"username"`
}

/*
Container is the message an interaction happened on.
*/
type Container struct {
	Type      string `json:"type"`
	MessageTS string `json:"message_ts"`
	ChannelID string `json:"channel_id"`
	ThreadTS  string `json:"thread_ts"`
}

/*
BlockAction is one pressed button or chosen option. Selected is the
option chosen from a menu.
*/
type BlockAction struct {
	ActionID string `json:"action_id"`
	BlockID  string `json:"block_id"`
	Value    string `json:"value"`
	ActionTS string `json:"action_ts"`
	Selected struct {
		Value string `json:"value"`
	} `json:"selected_option"`
}

/*
ParseInteraction reads an interaction payload.
*/
func ParseInteraction(raw []byte) (Interaction, error) {
	var in Interaction
	if err := json.Unmarshal(raw, &in); err != nil {
		return in, fmt.Errorf("read slack interaction: %w", err)
	}

	if in.CallbackID == "" && in.View != nil {
		in.CallbackID = in.View.CallbackID
	}

	return in, nil
}

/*
TeamID returns the team the interaction happened in.
*/
func (in Interaction) TeamID() string {
	if in.Team.ID != "" {
		return in.Team.ID
	}

	return in.User.TeamID
}

/*
ChannelID returns the channel the interaction happened in.
*/
func (in Interaction) ChannelID() string {
	if in.Channel.ID != "" {
		return in.Channel.ID
	}

	return in.Container.ChannelID
}

/*
DedupeKey recognizes the interaction when Slack delivers it again.
*/
func (in Interaction) DedupeKey() string {
	parts := []string{"interaction", in.Type, in.TriggerID}
	for _, action := range in.Actions {
		parts = append(parts, action.ActionID, action.ActionTS)
	}

	return strings.Join(parts, ":")
}
