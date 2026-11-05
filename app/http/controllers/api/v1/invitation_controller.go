/*
|--------------------------------------------------------------------------
| Invitation Controller
|--------------------------------------------------------------------------
|
| GET /invitations lists the invitations of the workspace in play, the
| pending ones unless filter[status] asks for others. POST /invitations
| invites a batch of addresses and answers for each one, and DELETE
| /invitations/{invitation} withdraws one. Ids of other workspaces'
| invitations are simply not found.
|
*/

package v1

import (
	"context"

	"github.com/dimmihq/dimmi/framework/fault"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/i18n"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/invitations"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
InvitationsOutput is a page of invitations.
*/
type InvitationsOutput struct {
	Body fhttp.Collection[resources.Invitation]
}

/*
InvitationResultsOutput is what became of each address of a batch.
*/
type InvitationResultsOutput struct {
	Body fhttp.Resource[[]resources.InvitationResult]
}

/*
InvitationController serves the invitations of the workspace in play.
*/
type InvitationController struct {
	List    *invitations.ListInvitations
	Create  *invitations.CreateInvitations
	Revoke  *invitations.RevokeInvitation
	IDs     *ids.Registry
	Cursors *pagination.Cursors
}

/*
invitationPage is where a page of invitations starts, carried in its
cursor.
*/
type invitationPage struct {
	Offset int `json:"o"`
}

/*
Index lists a page of invitations, newest first.
*/
func (c *InvitationController) Index(ctx context.Context, _ *requests.NoInput) (*InvitationsOutput, error) {
	r, _ := fhttp.RequestFrom(ctx)

	query, err := pagination.Parse(r.URL.Query(), pagination.Allow{Filters: []string{"status"}})
	if err != nil {
		return nil, err
	}

	in := invitations.ListInvitationsInput{
		Membership: membership(ctx), Statuses: query.Filters["status"], Limit: query.Limit,
	}
	if len(in.Statuses) == 0 {
		in.Statuses = []string{string(enums.InvitationStatusPending)}
	}

	if !known(in.Statuses, enums.InvitationStatuses()) {
		return nil, pagination.ErrInvalidFilter
	}

	if query.Cursor != "" {
		var start invitationPage
		if err := c.Cursors.Decode(query.Cursor, &start); err != nil {
			return nil, err
		}

		in.Offset = start.Offset
	}

	page, err := c.List.Handle(ctx, in)
	if err != nil {
		return nil, err
	}

	out := pagination.Page[resources.Invitation]{Items: resources.NewInvitations(page.Invitations), Limit: in.Limit}
	if page.More {
		if out.NextCursor, err = c.Cursors.Encode(invitationPage{Offset: in.Offset + in.Limit}); err != nil {
			return nil, err
		}
	}

	return &InvitationsOutput{Body: fhttp.NewCollection(out)}, nil
}

/*
Store invites the batch and answers with what became of each address.
*/
func (c *InvitationController) Store(
	ctx context.Context, in *requests.StoreInvitationsRequest,
) (*InvitationResultsOutput, error) {
	invitees := make([]invitations.Invitee, 0, len(in.Body.Invitations))

	for _, item := range in.Body.Invitations {
		teams, err := c.teamIDs(item.TeamIDs)
		if err != nil {
			return nil, err
		}

		invitee := invitations.Invitee{Email: item.Email, Role: enums.Role(item.Role), TeamIDs: teams}
		if item.CustomRoleID != nil {
			id, err := roleID(c.IDs, *item.CustomRoleID)
			if err != nil {
				return nil, err
			}

			invitee.CustomRoleID = &id
		}

		invitees = append(invitees, invitee)
	}

	outcomes, err := c.Create.Handle(ctx, invitations.CreateInvitationsInput{
		Membership: membership(ctx), Inviter: caller(ctx).User, Invitees: invitees,
	})
	if err != nil {
		return nil, err
	}

	results := resources.NewInvitationResults(outcomes, func(err error) resources.InvitationError {
		code := fault.CodeOf(err)

		return resources.InvitationError{Code: code, Message: i18n.T(ctx, "errors."+code, nil)}
	})

	return &InvitationResultsOutput{Body: fhttp.NewResource(results)}, nil
}

/*
Destroy withdraws an invitation.
*/
func (c *InvitationController) Destroy(ctx context.Context, in *requests.InvitationRequest) (*EmptyOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixInvitation), in.Invitation)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	err = c.Revoke.Handle(ctx, invitations.ChangeInvitationInput{
		Membership: membership(ctx), Caller: caller(ctx).User, InvitationID: id,
	})
	if err != nil {
		return nil, err
	}

	return &EmptyOutput{}, nil
}

/*
teamIDs decodes the public ids of the teams an invitation joins; a
malformed one is not found.
*/
func (c *InvitationController) teamIDs(public []string) ([]ids.UUID, error) {
	teams := make([]ids.UUID, 0, len(public))

	for _, team := range public {
		id, err := c.IDs.Decode(string(models.PrefixTeam), team)
		if err != nil {
			return nil, exceptions.ErrNotFound
		}

		teams = append(teams, id)
	}

	return teams, nil
}
