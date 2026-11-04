/*
|--------------------------------------------------------------------------
| Inbox Preference Controller
|--------------------------------------------------------------------------
|
| Each member reads and saves how they arrange their own inbox. The
| preferences belong to the caller, so there is nothing to name in the
| path.
|
*/

package v1

import (
	"context"
	"encoding/json"
	"fmt"

	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
)

/*
InboxPreferencesOutput is a member's inbox preferences.
*/
type InboxPreferencesOutput struct {
	Body fhttp.Resource[resources.InboxPreferences]
}

/*
InboxPreferenceController serves the caller's inbox preferences.
*/
type InboxPreferenceController struct {
	Show   *conversations.ShowPreferences
	Update *conversations.UpdatePreferences
}

/*
Get answers with the caller's inbox preferences.
*/
func (c *InboxPreferenceController) Get(ctx context.Context, _ *requests.NoInput) (*InboxPreferencesOutput, error) {
	p, err := c.Show.Handle(ctx, conversations.PreferencesInput{Membership: membership(ctx)})
	if err != nil {
		return nil, err
	}

	return &InboxPreferencesOutput{Body: fhttp.NewResource(resources.NewInboxPreferences(p))}, nil
}

/*
Patch saves the parts of the caller's inbox preferences the body sends.
*/
func (c *InboxPreferenceController) Patch(
	ctx context.Context, in *requests.UpdateInboxPreferencesRequest,
) (*InboxPreferencesOutput, error) {
	update := conversations.UpdatePreferencesInput{
		Membership: membership(ctx), PinnedAttributes: in.Body.PinnedAttributes, MutationID: mutationID(ctx),
	}

	if in.Body.Sidebar != nil {
		raw, err := json.Marshal(in.Body.Sidebar)
		if err != nil {
			return nil, fmt.Errorf("encode sidebar: %w", err)
		}

		update.Sidebar = raw
	}

	if in.Body.Views != nil {
		update.Views = make(map[string]conversations.ViewPreference, len(in.Body.Views))
		for key, view := range in.Body.Views {
			update.Views[key] = conversations.ViewPreference(view)
		}
	}

	p, err := c.Update.Handle(ctx, update)
	if err != nil {
		return nil, err
	}

	return &InboxPreferencesOutput{Body: fhttp.NewResource(resources.NewInboxPreferences(p))}, nil
}
