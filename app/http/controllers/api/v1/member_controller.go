/*
|--------------------------------------------------------------------------
| Member Controller
|--------------------------------------------------------------------------
|
| GET /members lists the people of the workspace in play a page at a
| time, GET /members/{member} shows one, and PATCH /members/{member}
| changes their role or status, handing ownership over included. "me"
| names the caller's own membership. Ids of other workspaces' members
| are simply not found. include=teams adds the teams each member is in.
|
*/

package v1

import (
	"context"
	"slices"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/members"
	"github.com/dimmihq/dimmi/app/actions/teams"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
CursorKeyPurpose names the key that signs list cursors, so a client
cannot forge where a page starts.
*/
const CursorKeyPurpose = "list-cursors"

/*
MembersOutput is a page of members.
*/
type MembersOutput struct {
	Body fhttp.Collection[resources.WorkspaceMember]
}

/*
MemberOutput is one member.
*/
type MemberOutput struct {
	Body fhttp.Resource[resources.WorkspaceMember]
}

/*
MemberController serves the members of the workspace in play.
*/
type MemberController struct {
	ListMembers  *members.ListMembers
	ShowMember   *members.ShowMember
	UpdateMember *members.UpdateMember
	MemberTeams  *teams.ListMemberTeams
	IDs          *ids.Registry
	Cursors      *pagination.Cursors
}

/*
memberPage is where a page of members starts, carried in its cursor.
*/
type memberPage struct {
	Offset int `json:"o"`
}

/*
Index lists a page of members, by name or, with sort=-created_at,
newest first; filter[status], filter[role] and q narrow it.
*/
func (c *MemberController) Index(ctx context.Context, _ *requests.NoInput) (*MembersOutput, error) {
	r, _ := fhttp.RequestFrom(ctx)

	query, err := pagination.Parse(r.URL.Query(), pagination.Allow{
		Filters: []string{"status", "role"}, Sorts: []string{"name", "created_at"}, Includes: []string{"teams"},
	})
	if err != nil {
		return nil, err
	}

	in, err := c.listInput(ctx, query)
	if err != nil {
		return nil, err
	}

	page, err := c.ListMembers.Handle(ctx, in)
	if err != nil {
		return nil, err
	}

	out := pagination.Page[resources.WorkspaceMember]{
		Items: resources.NewWorkspaceMembers(page.Members), Limit: in.Limit,
	}
	if err := c.includeTeams(ctx, query, out.Items, memberIDs(page.Members)); err != nil {
		return nil, err
	}
	if page.More {
		if out.NextCursor, err = c.Cursors.Encode(memberPage{Offset: in.Offset + in.Limit}); err != nil {
			return nil, err
		}
	}

	return &MembersOutput{Body: fhttp.NewCollection(out)}, nil
}

/*
Show answers with one member, with their teams under include=teams.
*/
func (c *MemberController) Show(ctx context.Context, in *requests.ShowMemberRequest) (*MemberOutput, error) {
	r, _ := fhttp.RequestFrom(ctx)

	query, err := pagination.Parse(r.URL.Query(), pagination.Allow{Includes: []string{"teams"}})
	if err != nil {
		return nil, err
	}

	id, err := c.memberID(ctx, in.Member)
	if err != nil {
		return nil, err
	}

	row, err := c.ShowMember.Handle(ctx, members.ShowMemberInput{Membership: membership(ctx), MemberID: id})
	if err != nil {
		return nil, err
	}

	out := []resources.WorkspaceMember{resources.NewWorkspaceMember(row.WorkspaceMember, row.User)}
	if err := c.includeTeams(ctx, query, out, []ids.UUID{row.WorkspaceMember.ID}); err != nil {
		return nil, err
	}

	return &MemberOutput{Body: fhttp.NewResource(out[0])}, nil
}

/*
includeTeams adds to each shaped member the teams they are in, when the
query includes teams. The members and their ids come in the same order.
*/
func (c *MemberController) includeTeams(
	ctx context.Context, query pagination.Query, out []resources.WorkspaceMember, memberIDs []ids.UUID,
) error {
	if !slices.Contains(query.Includes, "teams") {
		return nil
	}

	byMember, err := c.MemberTeams.Handle(ctx, teams.ListMemberTeamsInput{
		Membership: membership(ctx), MemberIDs: memberIDs,
	})
	if err != nil {
		return err
	}

	for i, id := range memberIDs {
		summaries := resources.NewTeamSummaries(byMember[id])
		out[i].Teams = &summaries
	}

	return nil
}

/*
memberIDs returns the ids of the listed members, in order.
*/
func memberIDs(rows []models.ListWorkspaceMembersRow) []ids.UUID {
	out := make([]ids.UUID, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.WorkspaceMember.ID)
	}

	return out
}

/*
Update changes a member's role or status and answers with the member as
they now are.
*/
func (c *MemberController) Update(ctx context.Context, in *requests.UpdateMemberRequest) (*MemberOutput, error) {
	id, err := c.memberID(ctx, in.Member)
	if err != nil {
		return nil, err
	}

	update := members.UpdateMemberInput{
		Principal: caller(ctx), Membership: membership(ctx), MemberID: id,
		AssignedRole: (*enums.Role)(in.Body.AssignedRole), Status: (*enums.MemberStatus)(in.Body.Status),
		KeepOwner: in.Body.KeepOwner != nil && *in.Body.KeepOwner,
	}

	if in.Body.ReassignToMemberID != nil {
		reassign, err := c.IDs.Decode(string(models.PrefixMember), *in.Body.ReassignToMemberID)
		if err != nil {
			return nil, exceptions.ErrReassignTargetInvalid
		}

		update.ReassignTo = &reassign
	}

	row, err := c.UpdateMember.Handle(ctx, update)
	if err != nil {
		return nil, err
	}

	return &MemberOutput{Body: fhttp.NewResource(resources.NewWorkspaceMember(row.WorkspaceMember, row.User))}, nil
}

/*
memberID returns the member a path names, "me" being the caller.
*/
func (c *MemberController) memberID(ctx context.Context, named string) (ids.UUID, error) {
	if named == requests.CurrentMember {
		return membership(ctx).Member.ID, nil
	}

	id, err := c.IDs.Decode(string(models.PrefixMember), named)
	if err != nil {
		return ids.Nil(), exceptions.ErrNotFound
	}

	return id, nil
}

/*
listInput turns the list query into the action's input, refusing
filter values no member can have.
*/
func (c *MemberController) listInput(ctx context.Context, query pagination.Query) (members.ListMembersInput, error) {
	in := members.ListMembersInput{
		Membership: membership(ctx), Statuses: query.Filters["status"], Roles: query.Filters["role"],
		Search: query.Search, Limit: query.Limit,
	}

	for _, sort := range query.Sorts {
		in.NewestFirst = sort.Field == "created_at" && sort.Desc
	}

	if !known(in.Statuses, enums.MemberStatuses()) || !known(in.Roles, enums.Roles()) {
		return in, pagination.ErrInvalidFilter
	}

	if query.Cursor != "" {
		var start memberPage
		if err := c.Cursors.Decode(query.Cursor, &start); err != nil {
			return in, err
		}

		in.Offset = start.Offset
	}

	return in, nil
}

/*
known reports whether every value is one of the allowed ones.
*/
func known[T ~string](values []string, allowed []T) bool {
	for _, value := range values {
		if !slices.Contains(allowed, T(value)) {
			return false
		}
	}

	return true
}
