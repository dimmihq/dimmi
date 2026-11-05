/*
|--------------------------------------------------------------------------
| Member Access And Availability
|--------------------------------------------------------------------------
|
| The parts of a member update that say which conversations the member
| sees and how they take new ones, read from the request, and the teams
| a member's access names added to each member shown. Teams travel by
| public id; one that cannot be read names no team.
|
*/

package v1

import (
	"context"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/availability"
	"github.com/dimmihq/dimmi/app/actions/members"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
accessChange reads the conversation access fields of an update, nil when
it names none. A team id that cannot be read names no team.
*/
func (c *MemberController) accessChange(b requests.UpdateMemberBody) (*members.AccessChange, error) {
	if b.Access == nil && b.AccessUnassigned == nil && b.AccessTeamIDs == nil {
		return nil, nil
	}

	change := &members.AccessChange{
		Mode: (*enums.ConversationAccess)(b.Access), Unassigned: b.AccessUnassigned,
	}
	if b.AccessTeamIDs == nil {
		return change, nil
	}

	teamIDs := make([]ids.UUID, 0, len(*b.AccessTeamIDs))
	for _, public := range *b.AccessTeamIDs {
		id, err := c.IDs.Decode(string(models.PrefixTeam), public)
		if err != nil {
			return nil, exceptions.ErrAccessTeamNotFound
		}

		teamIDs = append(teamIDs, id)
	}

	change.TeamIDs = &teamIDs

	return change, nil
}

/*
availabilityChange reads the availability fields of an update, nil when
it names none. A capacity sent as null lifts the limit.
*/
func availabilityChange(b requests.UpdateMemberBody, raw []byte) *availability.Change {
	clearCapacity := requests.Nulls(raw)["conversation_capacity"]
	if b.Availability == nil && b.AwayReassign == nil && b.AssignOnReply == nil && b.Capacity == nil &&
		b.CapacityByChannel == nil && !clearCapacity {
		return nil
	}

	change := &availability.Change{
		Availability: (*enums.Availability)(b.Availability), AwayReassign: b.AwayReassign,
		AssignOnReply: (*enums.AssignOnReply)(b.AssignOnReply), CapacityByChannel: b.CapacityByChannel,
	}

	if b.Capacity != nil || clearCapacity {
		change.Capacity = &availability.Limit{Max: b.Capacity}
	}

	return change
}

/*
includeAccessTeams adds to each shaped member the teams their
conversation access names. The shaped members and the rows come in the
same order, and only modes that read a list cost a query.
*/
func (c *MemberController) includeAccessTeams(
	ctx context.Context, out []resources.WorkspaceMember, rows []models.WorkspaceMember,
) error {
	byMember, err := c.AccessTeams.Handle(ctx, members.ListAccessTeamsInput{Membership: membership(ctx), Members: rows})
	if err != nil {
		return err
	}

	for i, row := range rows {
		out[i].AccessTeamIDs = resources.PublicTeamIDs(byMember[row.ID])
	}

	return nil
}
