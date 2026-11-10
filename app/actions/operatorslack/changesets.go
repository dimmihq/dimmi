/*
|--------------------------------------------------------------------------
| Operator's Changes In Slack
|--------------------------------------------------------------------------
|
| A change Operator proposes comes to the Slack thread as a card with
| Apply and Decline; an applied one offers Undo. Whoever presses decides
| as themselves, so a teammate who may not make the change is refused
| exactly as in the app, and only linked teammates decide at all. The
| card then says how it ended and who decided.
|
*/

package operatorslack

import (
	"context"
	"errors"
	"strings"

	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/actions/slackteam"
	"github.com/dimmihq/dimmi/app/auth/authz"
	slackdriver "github.com/dimmihq/dimmi/app/drivers/channel/slack"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ActionPrefix starts the action id of every changeset button.
*/
const ActionPrefix = "dimmi_operator:"

/*
The changeset buttons.
*/
const (
	ActionApply   = ActionPrefix + "apply"
	ActionDecline = ActionPrefix + "decline"
	ActionUndo    = ActionPrefix + "undo"
)

/*
Pressed is one press of a changeset button: the connection, the
interaction and the button.
*/
type Pressed struct {
	Connection  models.ChannelConnection
	Interaction slackdriver.Interaction
	Action      slackdriver.BlockAction
}

/*
ChangesetCard shows a change Operator proposed, with the buttons its
state allows. By names who decided, for a decided one.
*/
func ChangesetCard(c models.OperatorChangeset, by string) slackdriver.Card {
	id := models.PublicID(models.PrefixOperatorChangeset, c.ID)
	lines := []string{"*Proposed change:* " + slackdriver.Escape(c.Title)}

	if c.Reason != "" {
		lines = append(lines, slackdriver.Escape(c.Reason))
	}

	card := slackdriver.Card{Text: "Proposed change: " + c.Title, Lines: lines}

	switch c.Status {
	case "pending":
		card.Buttons = []slackdriver.Button{
			{ActionID: ActionApply, Label: "Apply", Value: id, Primary: true},
			{ActionID: ActionDecline, Label: "Decline", Value: id},
		}
	case "applied":
		card.Context = "Applied by " + slackdriver.Escape(by)
		card.Buttons = []slackdriver.Button{{ActionID: ActionUndo, Label: "Undo", Value: id}}
	default:
		card.Context = strings.ToUpper(c.Status[:1]) + c.Status[1:] + " by " + slackdriver.Escape(by)
	}

	return card
}

/*
OnAction decides a changeset as the teammate who pressed, reporting
whether the press was a changeset button.
*/
func (o *Operator) OnAction(ctx context.Context, p Pressed) (bool, error) {
	decide, ok := o.decision(p.Action.ActionID)
	if !ok {
		return false, nil
	}

	id, known := o.changeset(p.Action.Value)
	if !known {
		return true, nil
	}

	ctx = tenancy.With(ctx, p.Connection.WorkspaceID.String())
	team := slackteam.TeamOf(p.Connection, p.Interaction.User.TeamID, p.Interaction.Team.ID)

	who, linked, err := slackteam.Linked(ctx, o.DB, p.Connection, slackteam.User{
		TeamID: team, UserID: p.Interaction.User.ID,
	})
	if err != nil {
		return true, err
	}

	api, err := o.Bot(ctx, p.Connection)
	if err != nil {
		return true, err
	}

	notice := slackdriver.Notice{
		Channel: p.Interaction.ChannelID(), User: p.Interaction.User.ID, ThreadTS: p.Interaction.Container.ThreadTS,
		Text: slackteam.ConnectText(o.AppURL, "decide Operator's changes"),
	}

	if !linked {
		return true, api.Ephemeral(ctx, notice)
	}

	c, err := decide(ctx, who.Membership, id)
	if f, ok := errors.AsType[*fault.Error](err); ok {
		notice.Text = f.Message()

		return true, api.Ephemeral(ctx, notice)
	}

	if err != nil {
		return true, err
	}

	return true, api.UpdateCard(ctx, p.Interaction.ChannelID(), p.Interaction.Container.MessageTS,
		ChangesetCard(c, who.Name))
}

/*
decide is one way to decide a changeset as a teammate.
*/
type decide func(ctx context.Context, m authz.Membership, id ids.UUID) (models.OperatorChangeset, error)

/*
decision returns how the button decides.
*/
func (o *Operator) decision(actionID string) (decide, bool) {
	switch actionID {
	case ActionApply:
		return o.Changesets.Apply, true
	case ActionDecline:
		return o.Changesets.Decline, true
	case ActionUndo:
		return o.Changesets.Revert, true
	}

	return nil, false
}

/*
changeset reads a changeset's public id, reporting false for one that
is not.
*/
func (o *Operator) changeset(public string) (ids.UUID, bool) {
	id, err := o.IDs.Decode(string(models.PrefixOperatorChangeset), public)

	return id, err == nil
}
