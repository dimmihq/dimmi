/*
|--------------------------------------------------------------------------
| Broadcast Preferences Outputs
|--------------------------------------------------------------------------
|
| What the preference page reads: the workspace, the address, whether
| the person left every broadcast and each topic they may keep. It names
| only the link's own recipient.
|
*/

package v1

import (
	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/actions/broadcasts"
	"github.com/dimmihq/dimmi/app/models"
)

/*
BroadcastPreferences is what the page shows.
*/
type BroadcastPreferences struct {
	Object    string               `json:"object"`
	Workspace string               `json:"workspace"`
	Email     string               `json:"email"`
	OptedOut  bool                 `json:"unsubscribed_all"`
	Topics    []BroadcastTopicPick `json:"topics"`
}

/*
BroadcastTopicPick is one topic and whether the person gets it.
*/
type BroadcastTopicPick struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Subscribed  bool   `json:"subscribed"`
}

/*
BroadcastPreferencesOutput answers the page.
*/
type BroadcastPreferencesOutput struct {
	Body fhttp.Resource[BroadcastPreferences]
}

func broadcastPreferencesOutput(p broadcasts.Preferences, err error) (*BroadcastPreferencesOutput, error) {
	if err != nil {
		return nil, err
	}

	out := BroadcastPreferences{
		Object: "broadcast_preferences", Workspace: p.Workspace, Email: p.Email, OptedOut: p.OptedOut,
		Topics: []BroadcastTopicPick{},
	}

	for _, t := range p.Topics {
		out.Topics = append(out.Topics, BroadcastTopicPick{
			ID: models.PublicID(models.PrefixBroadcastTopic, t.ID), Name: t.Name, Description: t.Description,
			Subscribed: t.Subscribed,
		})
	}

	return &BroadcastPreferencesOutput{Body: fhttp.NewResource(out)}, nil
}
