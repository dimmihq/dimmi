/*
|--------------------------------------------------------------------------
| Membership Controller
|--------------------------------------------------------------------------
|
| GET /user/memberships lists the workspaces the caller belongs to, the
| one used last first, and DELETE /user/memberships/{membership} leaves
| one. Both are the user's own business, so neither needs a workspace
| header and leaving works even where the workspace's access rules
| would stop everything else.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/members"
	"github.com/dimmihq/dimmi/app/actions/workspaces"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
MembershipsOutput is the list of the caller's memberships.
*/
type MembershipsOutput struct {
	Body fhttp.Collection[resources.Membership]
}

/*
MembershipController serves the caller's memberships.
*/
type MembershipController struct {
	List  *workspaces.ListMemberships
	Leave *members.LeaveWorkspace
	IDs   *ids.Registry
}

/*
Index lists the caller's memberships. The trashed filter adds
workspaces in the trash ("with") or lists only them ("only").
*/
func (c *MembershipController) Index(ctx context.Context, _ *requests.NoInput) (*MembershipsOutput, error) {
	r, _ := fhttp.RequestFrom(ctx)

	query, err := pagination.Parse(r.URL.Query(), pagination.Allow{Filters: []string{"trashed"}})
	if err != nil {
		return nil, err
	}

	trashed, err := trashedFilter(query.Filters["trashed"])
	if err != nil {
		return nil, err
	}

	rows, err := c.List.Handle(ctx, workspaces.ListMembershipsInput{UserID: caller(ctx).User.ID, Trashed: trashed})
	if err != nil {
		return nil, err
	}

	items := resources.NewMemberships(rows)
	page := pagination.Page[resources.Membership]{Items: items, Limit: len(items)}

	return &MembershipsOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
Destroy leaves the workspace of one of the caller's memberships.
*/
func (c *MembershipController) Destroy(
	ctx context.Context, in *requests.DestroyMembershipRequest,
) (*EmptyOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixMember), in.Membership)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	err = c.Leave.Handle(ctx, members.LeaveWorkspaceInput{UserID: caller(ctx).User.ID, MemberID: id})
	if err != nil {
		return nil, err
	}

	return &EmptyOutput{}, nil
}

/*
trashedFilter reads filter[trashed], which takes one of with and only.
*/
func trashedFilter(values []string) (string, error) {
	if len(values) == 0 {
		return workspaces.TrashedWithout, nil
	}

	if len(values) == 1 && (values[0] == workspaces.TrashedWith || values[0] == workspaces.TrashedOnly) {
		return values[0], nil
	}

	return "", pagination.ErrInvalidFilter.With("parameter", "filter[trashed]")
}
