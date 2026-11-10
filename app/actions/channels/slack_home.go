/*
|--------------------------------------------------------------------------
| Slack App Home
|--------------------------------------------------------------------------
|
| In a helpdesk, colleagues ask the team for help in a channel, and the
| app's home tab is where each of them sees their own requests: open
| and recent ones, where each stands, and a way back to its thread. It
| is drawn fresh each time they open it, from their own conversations on
| the connection and nobody else's.
|
*/

package channels

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"

	slackdriver "github.com/dimmihq/dimmi/app/drivers/channel/slack"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
homeItems is how many requests the home tab lists.
*/
const homeItems = 10

/*
onHomeOpened draws the home tab for the person who opened it.
*/
func (s *Slack) onHomeOpened(ctx context.Context, ev SlackEvent) (bool, error) {
	if ev.Event.Tab != "home" || ev.Event.User == "" {
		return false, nil
	}

	user, err := s.profile(ctx, ev.Connection, ev.Event.User)
	if err != nil || user.IsBot {
		return false, err
	}

	items, err := s.requestsOf(ctx, ev.Connection, slackdriver.UserIdentity(user.TeamID, user.ID))
	if err != nil {
		return false, err
	}

	api, err := s.api(ctx, ev.Connection)
	if err != nil {
		return false, err
	}

	return true, api.PublishHome(ctx, ev.Event.User, items)
}

/*
requestsOf lists the person's newest conversations on the connection.
*/
func (s *Slack) requestsOf(ctx context.Context, conn models.ChannelConnection, identity string) (
	[]slackdriver.HomeItem, error,
) {
	var convs []models.Conversation

	err := s.read(ctx, func(q *models.Queries) error {
		contact, err := q.FindContactByIdentity(ctx, models.FindContactByIdentityParams{
			WorkspaceID: conn.WorkspaceID, Type: string(enums.IdentityTypeSlackUser), Value: identity,
		})
		if err != nil {
			return err
		}

		convs, err = q.ListContactConnectionConversations(ctx, models.ListContactConnectionConversationsParams{
			WorkspaceID: conn.WorkspaceID, ChannelConnectionID: conn.ID, ContactID: contact.ID, RowLimit: homeItems,
		})

		return err
	})
	if errors.Is(err, database.ErrNotFound) {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("list requests: %w", err)
	}

	items := make([]slackdriver.HomeItem, 0, len(convs))
	for _, conv := range convs {
		items = append(items, s.homeItem(ctx, conv))
	}

	return items, nil
}

/*
homeItem is one conversation as the home tab lists it.
*/
func (s *Slack) homeItem(ctx context.Context, conv models.Conversation) slackdriver.HomeItem {
	item := slackdriver.HomeItem{Title: fmt.Sprintf("Request #%d", conv.Number), Status: conv.Status}
	if conv.Subject != nil && *conv.Subject != "" {
		item.Title = *conv.Subject
	}

	thread, ok := threadOf(conv)
	if !ok || thread.Direct() {
		return item
	}

	if room, found, err := s.roomOfConversation(ctx, conv); err == nil && found {
		item.Link = "https://slack.com/archives/" + room.ExternalID + "/p" + strings.ReplaceAll(thread.RootTS, ".", "")
	}

	return item
}
