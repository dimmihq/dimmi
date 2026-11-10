/*
|--------------------------------------------------------------------------
| Alert Buttons In Slack
|--------------------------------------------------------------------------
|
| A teammate presses Take it, Snooze until tomorrow or Mark urgent on an
| alert in Slack, and the change runs as them, through the same action
| the app uses, so nobody does from Slack what they could not do in the
| app. The alert then changes in place to say who did what, so the rest
| of the channel sees it is handled. A Slack user not linked to a
| teammate is asked to connect their account, and nothing changes.
|
*/

package notifications

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/actions/slackteam"
	"github.com/dimmihq/dimmi/app/auth/authz"
	slackdriver "github.com/dimmihq/dimmi/app/drivers/channel/slack"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
SlackPress is one press of an alert button: the connection it arrived
on, the interaction and the button pressed.
*/
type SlackPress struct {
	Connection  models.ChannelConnection
	Interaction slackdriver.Interaction
	Action      slackdriver.BlockAction
}

/*
SlackButtons handles presses of alert buttons. Bot opens Slack with the
connection's bot token, and AppURL is the install's address.
*/
type SlackButtons struct {
	DB      *database.DB
	Clock   clock.Clock
	IDs     *ids.Registry
	Changes *conversations.ApplyConversationChanges
	Bot     func(ctx context.Context, conn models.ChannelConnection) (*slackdriver.API, error)
	Links   Links
	AppURL  string
}

/*
Handle runs the press, reporting whether it was an alert button.
*/
func (b *SlackButtons) Handle(ctx context.Context, p SlackPress) (bool, error) {
	if p.Action.ActionID == SlackActionOpen {
		return true, nil
	}

	change, done, ok := changeOf(p.Action.ActionID)
	if !ok {
		return false, nil
	}

	conv, known := decode(b.IDs, models.PrefixConversation, p.Action.Value)
	if !known {
		return true, nil
	}

	ctx = tenancy.With(ctx, p.Connection.WorkspaceID.String())
	team := slackteam.TeamOf(p.Connection, p.Interaction.User.TeamID, p.Interaction.Team.ID)

	who, linked, err := slackteam.Linked(ctx, b.DB, p.Connection, slackteam.User{
		TeamID: team, UserID: p.Interaction.User.ID,
	})
	if err != nil {
		return true, err
	}

	if !linked {
		b.tell(ctx, p, slackteam.ConnectText(b.AppURL, "act on alerts from Slack"))

		return true, nil
	}

	return true, b.apply(ctx, p, who, pressed{conversation: conv, change: change(who.Membership), done: done})
}

/*
pressed is what a press asks: the conversation, its change and the
words the alert says once it is done.
*/
type pressed struct {
	conversation ids.UUID
	change       conversations.Changes
	done         string
}

/*
changeOf returns the change a button asks for and how it reads once
done.
*/
func changeOf(actionID string) (func(authz.Membership) conversations.Changes, string, bool) {
	switch actionID {
	case SlackActionTake:
		return func(m authz.Membership) conversations.Changes {
			return conversations.Changes{AssignMember: true, Assign: conversations.Assignment{Member: &m.Member.ID}}
		}, "Taken by", true
	case SlackActionSnooze:
		return func(authz.Membership) conversations.Changes {
			return conversations.Changes{SnoozePreset: conversations.PresetTomorrow}
		}, "Snoozed until tomorrow by", true
	case SlackActionUrgent:
		return func(authz.Membership) conversations.Changes {
			urgent := enums.ConversationPriorityUrgent

			return conversations.Changes{Attributes: conversations.Attributes{Priority: &urgent}}
		}, "Marked urgent by", true
	}

	return nil, "", false
}

/*
apply runs the change as the teammate and updates the alert in place;
a change the teammate may not make is explained to them alone.
*/
func (b *SlackButtons) apply(ctx context.Context, p SlackPress, who slackteam.Teammate, in pressed) error {
	applied, err := b.Changes.Handle(ctx, conversations.ApplyChangesInput{
		Membership: who.Membership, Ref: conversations.Ref{ID: in.conversation}, Changes: in.change,
	})
	if f, ok := errors.AsType[*fault.Error](err); ok {
		b.tell(ctx, p, f.Message())

		return nil
	}

	if err != nil {
		return fmt.Errorf("apply slack alert action: %w", err)
	}

	api, err := b.Bot(ctx, p.Connection)
	if err != nil {
		return err
	}

	c := applied.Conversation.Conversation
	link := b.Links.Conversation(who.Membership.Workspace.Slug, c.ID)
	status := in.done + " " + who.Name
	card := slackdriver.Card{
		Text: status,
		Lines: []string{
			"*" + slackdriver.LinkText(link, fmt.Sprintf("#%d", c.Number)) + "* " + subjectOf(c),
			in.done + " *" + slackdriver.Escape(who.Name) + "*",
		},
		Buttons: []slackdriver.Button{
			{ActionID: SlackActionOpen, Label: "Open in dimmi", URL: link, Value: p.Action.Value},
		},
	}

	return api.UpdateCard(ctx, p.Interaction.ChannelID(), p.Interaction.Container.MessageTS, card)
}

/*
tell shows the teammate a message only they see.
*/
func (b *SlackButtons) tell(ctx context.Context, p SlackPress, text string) {
	api, err := b.Bot(ctx, p.Connection)
	if err != nil {
		return
	}

	_ = api.Ephemeral(ctx, slackdriver.Notice{
		Channel: p.Interaction.ChannelID(), User: p.Interaction.User.ID, ThreadTS: p.Interaction.Container.ThreadTS,
		Text: text,
	})
}

/*
subjectOf is a conversation's subject, escaped, or nothing.
*/
func subjectOf(c models.Conversation) string {
	if c.Subject == nil {
		return ""
	}

	return slackdriver.Escape(*c.Subject)
}

/*
decode reads a public id of the kind, reporting false for one that is
not.
*/
func decode(registry *ids.Registry, prefix models.Prefix, public string) (ids.UUID, bool) {
	id, err := registry.Decode(string(prefix), public)

	return id, err == nil
}
