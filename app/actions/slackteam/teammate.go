/*
|--------------------------------------------------------------------------
| Slack Teammates
|--------------------------------------------------------------------------
|
| Whatever a teammate does from Slack, a button on an alert or a question
| to Operator, runs as their own membership: their role, the custom role
| that narrows it and the conversations they may see, exactly as in the
| app. Only a Slack user linked to an active member counts; anyone else
| is told how to connect their account, and nothing runs.
|
*/

package slackteam

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	slackdriver "github.com/dimmihq/dimmi/app/drivers/channel/slack"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ConnectPath is the settings page where a teammate connects their Slack
account.
*/
const ConnectPath = "/d/settings/integrations"

/*
ConnectText asks a Slack user who is not linked to connect their
account, linking the settings at the install's address.
*/
func ConnectText(appURL, doing string) string {
	link := slackdriver.LinkText(strings.TrimRight(appURL, "/")+ConnectPath, "Connect your Slack account")

	return link + " in dimmi to " + doing + "."
}

/*
Teammate is a linked teammate acting from Slack: their membership and
their name.
*/
type Teammate struct {
	Membership authz.Membership
	Name       string
}

/*
User is a Slack user: their home team and their id.
*/
type User struct {
	TeamID string
	UserID string
}

/*
Linked returns the active teammate the Slack user is linked to on the
connection, reporting false when they are not linked. The context
carries the connection's workspace.
*/
func Linked(ctx context.Context, db *database.DB, conn models.ChannelConnection, user User) (Teammate, bool, error) {
	var (
		t      Teammate
		linked bool
	)

	err := db.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		link, err := q.FindChannelMemberLink(ctx, models.FindChannelMemberLinkParams{
			WorkspaceID: conn.WorkspaceID, ChannelConnectionID: conn.ID,
			ExternalUserID: slackdriver.UserIdentity(user.TeamID, user.UserID),
		})
		if errors.Is(err, database.ErrNotFound) {
			return nil
		}

		if err != nil {
			return fmt.Errorf("find member link: %w", err)
		}

		t.Name = link.MemberName
		t.Membership, linked, err = Membership(ctx, q, conn.WorkspaceID, link.WorkspaceMemberID)

		return err
	})

	return t, linked, err
}

/*
Membership returns the active member's membership of the workspace with
their effective permissions and conversation access, reporting false
for a member who is no longer active.
*/
func Membership(ctx context.Context, q *models.Queries, ws, memberID ids.UUID) (authz.Membership, bool, error) {
	w, err := q.FindWorkspace(ctx, ws)
	if err != nil {
		return authz.Membership{}, false, fmt.Errorf("find workspace: %w", err)
	}

	member, err := q.FindWorkspaceMember(ctx, models.FindWorkspaceMemberParams{WorkspaceID: ws, ID: memberID})
	if errors.Is(err, database.ErrNotFound) {
		return authz.Membership{}, false, nil
	}

	if err != nil {
		return authz.Membership{}, false, fmt.Errorf("find member: %w", err)
	}

	if member.Status != string(enums.MemberStatusActive) || w.DeletedAt != nil || w.SuspendedAt != nil {
		return authz.Membership{}, false, nil
	}

	custom, err := q.CustomPermissions(ctx, member)
	if err != nil {
		return authz.Membership{}, false, fmt.Errorf("load custom role: %w", err)
	}

	m := authz.NewMembership(w, member, authz.HeldPermissions(enums.Role(member.Role), custom))

	if m.AccessTeams, err = authz.LoadAccessTeams(ctx, q, member); err != nil {
		return m, false, fmt.Errorf("load access teams: %w", err)
	}

	if m.HiddenInboxes, err = authz.LoadHiddenInboxes(ctx, q, member); err != nil {
		return m, false, fmt.Errorf("load hidden inboxes: %w", err)
	}

	return m, true, nil
}

/*
TeamOf is the Slack team a user acting on the connection belongs to:
the one Slack named, else the connection's own.
*/
func TeamOf(conn models.ChannelConnection, named ...string) string {
	for _, team := range named {
		if team != "" {
			return team
		}
	}

	var facts struct {
		TeamID string `json:"team_id"`
	}

	_ = json.Unmarshal(conn.Config, &facts)

	return facts.TeamID
}
