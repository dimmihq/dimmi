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
	"errors"
	"fmt"
	"slices"

	"github.com/dimmihq/dimmi/framework/database"
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
messengerOf returns the messenger the help center's pages load and
whether the workspace's AI agent answers it, the zero value when the
pages show none. A switched-off messenger shows nothing, so the first
live one stands in for a picked one that is off.
*/
func messengerOf(ctx context.Context, q *models.Queries, h models.HelpCenter) (Messenger, error) {
	s := SettingsOf(h)
	if !s.MessengerOn() {
		return Messenger{}, nil
	}
	all, err := q.ListWidgetSettings(ctx, h.WorkspaceID)
	if err != nil {
		return Messenger{}, fmt.Errorf("list messengers: %w", err)
	}
	live := slices.DeleteFunc(all, func(w models.WidgetSetting) bool { return !w.Enabled })
	if len(live) == 0 {
		return Messenger{}, nil
	}
	shown := live[0]
	for _, w := range live {
		if models.PublicID(models.PrefixWidget, w.ID) == s.MessengerID {
			shown = w
		}
	}
	ai, err := agentAnswers(ctx, q, shown)
	return Messenger{ID: models.PublicID(models.PrefixWidget, shown.ID), AI: ai}, err
}

/*
agentAnswers reports whether the workspace's AI agent answers chats from
the messenger: it is on, open to the messenger and not kept out of this
one.
*/
func agentAnswers(ctx context.Context, q *models.Queries, w models.WidgetSetting) (bool, error) {
	agent, err := q.FindAgent(ctx, w.WorkspaceID)
	if errors.Is(err, database.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("find agent: %w", err)
	}
	open := len(agent.EnabledChannels) == 0 || slices.Contains(agent.EnabledChannels, "widget")
	return agent.Enabled && open && !slices.Contains(agent.ExcludedChannelConnectionIds, w.ChannelConnectionID), nil
}

/*
Messenger is the messenger a help center's pages load: its public ID,
and AI when the workspace's AI agent answers it.
*/
type Messenger struct {
	ID string
	AI bool
}
