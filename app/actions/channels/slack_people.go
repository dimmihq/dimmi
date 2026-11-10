/*
|--------------------------------------------------------------------------
| Slack People
|--------------------------------------------------------------------------
|
| Who wrote a message decides what it is. In a shared channel the other
| organization's people are customers and the team's own are teammates,
| except guests, who are customers. In a community channel everyone is a
| customer unless linked to a teammate. In a helpdesk channel the team's
| colleagues ask, so only linked teammates answer. Bots never count:
| their messages would set off loops between apps.
|
| A teammate is linked the first time they write when their Slack email
| is the verified sign-in address of an active member, so most teams
| never link anyone by hand. Profiles are read once a day at most.
|
*/

package channels

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	slackdriver "github.com/dimmihq/dimmi/app/drivers/channel/slack"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
profileTTL is how long a Slack profile is trusted before it is read
again.
*/
const profileTTL = 24 * time.Hour

/*
authorKind is what the author of a message is to the conversation.
*/
type authorKind string

/*
The author kinds: a customer, a teammate of the workspace, and someone
whose messages are never stored, such as a bot.
*/
const (
	authorContact  authorKind = "contact"
	authorTeammate authorKind = "teammate"
	authorIgnored  authorKind = "ignored"
)

/*
author is who wrote a message: what they are, their Slack profile and,
for a linked teammate, the member and their name.
*/
type author struct {
	kind       authorKind
	user       slackdriver.User
	identity   string
	memberID   *ids.UUID
	memberName string
}

/*
classify tells who wrote the message in the room, nil for a direct
message.
*/
func (s *Slack) classify(ctx context.Context, conn models.ChannelConnection, room *models.ChannelRoom,
	userID string,
) (author, error) {
	user, err := s.profile(ctx, conn, userID)
	if err != nil {
		return author{}, err
	}

	a := author{user: user, identity: slackdriver.UserIdentity(user.TeamID, user.ID)}
	if user.IsBot {
		a.kind = authorIgnored

		return a, nil
	}

	if err := s.link(ctx, conn, &a); err != nil {
		return a, err
	}

	a.kind = kindOf(FactsOf(conn).TeamID, room, a)

	return a, nil
}

/*
kindOf decides what the author is in the room by its mode.
*/
func kindOf(installTeam string, room *models.ChannelRoom, a author) authorKind {
	if a.memberID != nil {
		return authorTeammate
	}

	if room == nil {
		return authorContact
	}

	if room.Mode == string(enums.RoomModeShared) && a.user.TeamID == installTeam && !a.user.Guest() {
		return authorTeammate
	}

	return authorContact
}

/*
link finds the teammate the Slack user is linked to, linking them first
when their Slack email is an active member's verified address in the
installing team.
*/
func (s *Slack) link(ctx context.Context, conn models.ChannelConnection, a *author) error {
	return s.read(ctx, func(q *models.Queries) error {
		found, err := q.FindChannelMemberLink(ctx, models.FindChannelMemberLinkParams{
			WorkspaceID: conn.WorkspaceID, ChannelConnectionID: conn.ID, ExternalUserID: a.identity,
		})
		if err == nil {
			a.memberID, a.memberName = &found.WorkspaceMemberID, found.MemberName

			return nil
		}

		if !errors.Is(err, database.ErrNotFound) {
			return fmt.Errorf("find member link: %w", err)
		}

		if a.user.Email == "" || a.user.TeamID != FactsOf(conn).TeamID || a.user.Guest() {
			return nil
		}

		return s.linkByEmail(ctx, q, conn, a)
	})
}

/*
linkByEmail links the Slack user to the active member whose verified
address their Slack email is.
*/
func (s *Slack) linkByEmail(ctx context.Context, q *models.Queries, conn models.ChannelConnection, a *author) error {
	member, err := q.FindActiveMemberByVerifiedEmail(ctx, models.FindActiveMemberByVerifiedEmailParams{
		WorkspaceID: conn.WorkspaceID, Email: a.user.Email,
	})
	if errors.Is(err, database.ErrNotFound) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("find member by email: %w", err)
	}

	_, err = q.CreateChannelMemberLink(ctx, models.CreateChannelMemberLinkParams{
		WorkspaceID: conn.WorkspaceID, ChannelConnectionID: conn.ID, WorkspaceMemberID: member.ID,
		ExternalUserID: a.identity, Source: "email_match", Now: s.Clock.Now(),
	})
	if errors.Is(err, database.ErrNotFound) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("link member: %w", err)
	}

	a.memberID, a.memberName = &member.ID, member.Name

	return nil
}

/*
profile returns the Slack user's profile, from the cache when it was
read within a day.
*/
func (s *Slack) profile(ctx context.Context, conn models.ChannelConnection, userID string) (slackdriver.User, error) {
	key := "slack:user:" + conn.ID.String() + ":" + userID

	var user slackdriver.User

	if raw, err := s.Cache.Get(ctx, key); err == nil && json.Unmarshal(raw, &user) == nil {
		return user, nil
	}

	api, err := s.api(ctx, conn)
	if err != nil {
		return user, err
	}

	user, err = api.User(ctx, userID)
	if err != nil {
		return user, err
	}

	if raw, err := json.Marshal(user); err == nil {
		_ = s.Cache.Set(ctx, key, raw, profileTTL)
	}

	return user, nil
}

/*
names resolves mentioned users to their names, for Markdown.
*/
func (s *Slack) names(ctx context.Context, conn models.ChannelConnection) slackdriver.Names {
	return func(userID string) string {
		user, err := s.profile(ctx, conn, userID)
		if err != nil {
			return ""
		}

		return user.Name
	}
}
