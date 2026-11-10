/*
|--------------------------------------------------------------------------
| Slack App Home
|--------------------------------------------------------------------------
|
| In a helpdesk, colleagues ask the team for help in a channel, and the
| app's home tab is where each of them sees their own requests: open
| and recent ones, where each stands, and a way back to its thread. A
| linked teammate sees instead what they hold, each a link into the
| inbox. It is drawn fresh each time it is opened, from that person's
| own conversations and nobody else's.
|
*/

package channels

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

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

	home, err := s.homeOf(ctx, ev.Connection, user)
	if err != nil {
		return false, err
	}

	api, err := s.api(ctx, ev.Connection)
	if err != nil {
		return false, err
	}

	return true, api.PublishHome(ctx, ev.Event.User, home)
}

/*
homeOf is the App Home of the Slack user: what a linked teammate holds,
or what anyone else asked.
*/
func (s *Slack) homeOf(ctx context.Context, conn models.ChannelConnection, user slackdriver.User,
) (slackdriver.Home, error) {
	a := author{user: user, identity: slackdriver.UserIdentity(user.TeamID, user.ID)}
	if err := s.link(ctx, conn, &a); err != nil {
		return slackdriver.Home{}, err
	}

	if a.memberID != nil {
		items, err := s.assignedTo(ctx, conn, *a.memberID)

		return slackdriver.TeammateHome(items), err
	}

	items, err := s.requestsOf(ctx, conn, a.identity)

	return slackdriver.RequesterHome(items), err
}

/*
assignedTo lists the open conversations the teammate holds, each a link
into the inbox.
*/
func (s *Slack) assignedTo(ctx context.Context, conn models.ChannelConnection, memberID ids.UUID,
) ([]slackdriver.HomeItem, error) {
	var convs []models.Conversation

	err := s.read(ctx, func(q *models.Queries) error {
		var err error

		convs, err = q.ListMemberOpenConversations(ctx, models.ListMemberOpenConversationsParams{
			WorkspaceID: conn.WorkspaceID, MemberID: &memberID, RowLimit: homeItems,
		})

		return err
	})
	if err != nil {
		return nil, fmt.Errorf("list assigned conversations: %w", err)
	}

	items := make([]slackdriver.HomeItem, 0, len(convs))
	for _, conv := range convs {
		item := slackdriver.HomeItem{
			Title: fmt.Sprintf("#%d", conv.Number), Status: conv.Status,
			Link: s.conversationLink(conv.ID), LinkLabel: "Open in dimmi",
		}
		if conv.Subject != nil && *conv.Subject != "" {
			item.Title = *conv.Subject
		}

		items = append(items, item)
	}

	return items, nil
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
