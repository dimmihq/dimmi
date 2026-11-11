/*
|--------------------------------------------------------------------------
| Broadcast Preferences Controller
|--------------------------------------------------------------------------
|
| GET /broadcast-preferences/{token} reads one person's choices from the
| signed link in a broadcast email; PATCH saves them. Nobody signs in: the
| link is the proof, and it names only its own recipient.
|
*/

package v1

import (
	"context"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/broadcasts"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
BroadcastPreferencesController serves the preference page.
*/
type BroadcastPreferencesController struct {
	Broadcasts *broadcasts.Broadcasts
	IDs        *ids.Registry
}

/*
BroadcastPreferencesRequest names the link's token.
*/
type BroadcastPreferencesRequest struct {
	Token string `path:"token"`
}

/*
SaveBroadcastPreferencesRequest is the person's choices: the topics
they keep, or leaving everything.
*/
type SaveBroadcastPreferencesRequest struct {
	Token string `path:"token"`
	Body  struct {
		Topics []string `json:"topics"`
		All    bool     `json:"unsubscribe_all"`
	}
}

/*
Show reads the person's choices.
*/
func (c *BroadcastPreferencesController) Show(ctx context.Context, in *BroadcastPreferencesRequest,
) (*BroadcastPreferencesOutput, error) {
	return broadcastPreferencesOutput(c.Broadcasts.Preferences(ctx, in.Token))
}

/*
Update saves the person's choices.
*/
func (c *BroadcastPreferencesController) Update(ctx context.Context, in *SaveBroadcastPreferencesRequest,
) (*BroadcastPreferencesOutput, error) {
	keep := make([]ids.UUID, 0, len(in.Body.Topics))
	for _, public := range in.Body.Topics {
		id, err := c.IDs.Decode(string(models.PrefixBroadcastTopic), public)
		if err != nil {
			return nil, exceptions.ErrNotFound
		}

		keep = append(keep, id)
	}

	return broadcastPreferencesOutput(c.Broadcasts.SavePreferences(ctx, in.Token, keep, in.Body.All))
}
