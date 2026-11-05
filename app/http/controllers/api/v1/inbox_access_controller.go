/*
|--------------------------------------------------------------------------
| Inbox Access Controller
|--------------------------------------------------------------------------
|
| Saves who may open an inbox. The list arrives whole and replaces the
| one the inbox had, so a page that shows the list can save it as shown.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/inboxes"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
UpdateAccess saves who may open the inbox.
*/
func (c *InboxController) UpdateAccess(
	ctx context.Context, in *requests.UpdateInboxAccessRequest,
) (*InboxOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixInbox), in.Inbox)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	teams, err := c.decodeAll(models.PrefixTeam, in.Body.TeamIDs)
	if err != nil {
		return nil, exceptions.ErrInboxTeamNotFound.OnField("team_ids")
	}

	members, err := c.decodeAll(models.PrefixMember, in.Body.MemberIDs)
	if err != nil {
		return nil, exceptions.ErrInboxAccessMemberNotFound.OnField("member_ids")
	}

	inbox, err := c.Access.Handle(ctx, inboxes.SetInboxAccessInput{
		Membership: membership(ctx), InboxID: id,
		Access: inboxes.Access{Mode: in.Body.Mode, Teams: teams, Members: members},
	})
	if err != nil {
		return nil, err
	}

	return &InboxOutput{Body: fhttp.NewResource(resources.NewInbox(inbox))}, nil
}

/*
decodeAll turns public ids of one kind into internal ones.
*/
func (c *InboxController) decodeAll(prefix models.Prefix, public []string) ([]ids.UUID, error) {
	out := make([]ids.UUID, 0, len(public))
	for _, p := range public {
		id, err := c.IDs.Decode(string(prefix), p)
		if err != nil {
			return nil, err
		}

		out = append(out, id)
	}

	return out, nil
}
