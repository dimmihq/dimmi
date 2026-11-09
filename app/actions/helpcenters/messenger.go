/*
|--------------------------------------------------------------------------
| Help Center Messenger
|--------------------------------------------------------------------------
|
| A help center's pages show one of the workspace's messengers, so a
| reader who cannot find an answer can ask without leaving the page. The
| team picks which one; until they do, or once the one they picked is
| gone, the workspace's first messenger shows. A team that wants none
| turns it off.
|
*/

package helpcenters

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
)

/*
ErrMessengerUnknown refuses a messenger the workspace does not have.
*/
var ErrMessengerUnknown = fault.New(fault.Invalid, "messenger_unknown",
	"Pick one of this workspace's messengers.").OnField("settings.messenger_id")

/*
checkMessenger refuses a picked messenger that is not one of the
workspace's live messengers. An empty pick means the first one.
*/
func checkMessenger(ctx context.Context, q *models.Queries, workspaceID ids.UUID, picked string) error {
	if picked == "" {
		return nil
	}
	live, err := q.ListWidgetSettings(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("list messengers: %w", err)
	}
	for _, w := range live {
		if models.PublicID(models.PrefixWidget, w.ID) == picked {
			return nil
		}
	}
	return ErrMessengerUnknown
}

/*
messengerOf returns the public id of the messenger the help center's
pages load, empty when it shows none.
*/
func messengerOf(ctx context.Context, q *models.Queries, h models.HelpCenter) (string, error) {
	s := SettingsOf(h)
	if !s.MessengerOn() {
		return "", nil
	}
	live, err := q.ListWidgetSettings(ctx, h.WorkspaceID)
	if err != nil {
		return "", fmt.Errorf("list messengers: %w", err)
	}
	if len(live) == 0 {
		return "", nil
	}
	for _, w := range live {
		if id := models.PublicID(models.PrefixWidget, w.ID); id == s.MessengerID {
			return id, nil
		}
	}
	return models.PublicID(models.PrefixWidget, live[0].ID), nil
}
