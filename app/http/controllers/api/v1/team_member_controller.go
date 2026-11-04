/*
|--------------------------------------------------------------------------
| Team Member Controller
|--------------------------------------------------------------------------
|
| POST /teams/{team}/members puts up to a hundred members in a team and
| answers with one result per id sent, in the order sent, so a partial
| success is never silent. DELETE /teams/{team}/members/{member} takes
| one member out.
|
*/

package v1

import (
	"context"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/teams"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
TeamMemberResultsOutput is the result of adding each member asked for.
*/
type TeamMemberResultsOutput struct {
	Body struct {
		Data []resources.TeamMemberResult `json:"data"`
	}
}

/*
TeamMemberController serves the members of one team.
*/
type TeamMemberController struct {
	Add    *teams.AddTeamMembers
	Remove *teams.RemoveTeamMember
	IDs    *ids.Registry
}

/*
Store adds the members to the team. An id that names no member of the
workspace comes back as not_found.
*/
func (c *TeamMemberController) Store(
	ctx context.Context, in *requests.StoreTeamMembersRequest,
) (*TeamMemberResultsOutput, error) {
	team, err := teamID(c.IDs, in.Team)
	if err != nil {
		return nil, err
	}

	memberIDs := make([]ids.UUID, 0, len(in.Body.MemberIDs))
	for _, named := range in.Body.MemberIDs {
		id, err := c.IDs.Decode(string(models.PrefixMember), named)
		if err != nil {
			id = ids.Nil()
		}

		memberIDs = append(memberIDs, id)
	}

	results, err := c.Add.Handle(ctx, teams.AddTeamMembersInput{
		Membership: membership(ctx), TeamID: team, MemberIDs: memberIDs,
	})
	if err != nil {
		return nil, err
	}

	out := &TeamMemberResultsOutput{}
	out.Body.Data = make([]resources.TeamMemberResult, 0, len(results))

	for i, result := range results {
		out.Body.Data = append(out.Body.Data, resources.TeamMemberResult{
			MemberID: in.Body.MemberIDs[i], Result: result,
		})
	}

	return out, nil
}

/*
Destroy takes a member out of the team.
*/
func (c *TeamMemberController) Destroy(
	ctx context.Context, in *requests.DestroyTeamMemberRequest,
) (*EmptyOutput, error) {
	team, err := teamID(c.IDs, in.Team)
	if err != nil {
		return nil, err
	}

	member, err := c.IDs.Decode(string(models.PrefixMember), in.Member)
	if err != nil {
		member = ids.Nil()
	}

	err = c.Remove.Handle(ctx, teams.RemoveTeamMemberInput{Membership: membership(ctx), TeamID: team, MemberID: member})
	if err != nil {
		return nil, err
	}

	return &EmptyOutput{}, nil
}
