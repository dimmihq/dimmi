/*
|--------------------------------------------------------------------------
| Snooze Controller
|--------------------------------------------------------------------------
|
| The snooze lists of the workspace and its teams, and the choices one
| conversation offers with the moment each ends for the caller.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
maxSnoozeChoices is the most choices a conversation offers.
*/
const maxSnoozeChoices = 10

/*
SnoozeListsOutput is the snooze lists set.
*/
type SnoozeListsOutput struct {
	Body fhttp.Resource[resources.SnoozeLists]
}

/*
SnoozeChoicesOutput is a conversation's snooze choices.
*/
type SnoozeChoicesOutput struct {
	Body fhttp.Collection[resources.SnoozeChoice]
}

/*
SnoozeController serves snooze lists and choices.
*/
type SnoozeController struct {
	Lists   *conversations.ListSnoozeLists
	Update  *conversations.UpdateSnoozeList
	Options *conversations.ShowSnoozeOptions
	IDs     *ids.Registry
}

/*
Index answers with the snooze lists set.
*/
func (c *SnoozeController) Index(ctx context.Context, _ *requests.NoInput) (*SnoozeListsOutput, error) {
	lists, err := c.Lists.Handle(ctx, membership(ctx))
	if err != nil {
		return nil, err
	}

	return &SnoozeListsOutput{Body: fhttp.NewResource(resources.NewSnoozeLists(lists))}, nil
}

/*
Patch sets or removes the workspace's or a team's list, and answers with
every list.
*/
func (c *SnoozeController) Patch(
	ctx context.Context, in *requests.UpdateSnoozeListRequest,
) (*SnoozeListsOutput, error) {
	update := conversations.UpdateSnoozeListInput{Membership: membership(ctx)}

	if in.Scope != "workspace" {
		team, err := c.IDs.Decode(string(models.PrefixTeam), in.Scope)
		if err != nil {
			return nil, exceptions.ErrNotFound
		}

		update.Team = &team
	}

	if in.Body.Options != nil {
		update.Options = make([]conversations.SnoozeOption, len(in.Body.Options))
		for i, o := range in.Body.Options {
			update.Options[i] = conversations.SnoozeOption(o)
		}
	}

	if err := c.Update.Handle(ctx, update); err != nil {
		return nil, err
	}

	return c.Index(ctx, nil)
}

/*
Choices answers with the snooze choices of the conversation named by
the conversation query parameter.
*/
func (c *SnoozeController) Choices(ctx context.Context, _ *requests.NoInput) (*SnoozeChoicesOutput, error) {
	r, _ := fhttp.RequestFrom(ctx)

	ref, err := conversationRef(c.IDs, r.URL.Query().Get("conversation"))
	if err != nil {
		return nil, err
	}

	options, err := c.Options.Handle(ctx, conversations.SnoozeOptionsInput{Membership: membership(ctx), Ref: ref})
	if err != nil {
		return nil, err
	}

	page := pagination.Page[resources.SnoozeChoice]{Items: resources.NewSnoozeChoices(options), Limit: maxSnoozeChoices}

	return &SnoozeChoicesOutput{Body: fhttp.NewCollection(page)}, nil
}
