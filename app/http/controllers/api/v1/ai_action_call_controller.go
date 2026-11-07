/*
|--------------------------------------------------------------------------
| Agent Action Call Controller
|--------------------------------------------------------------------------
|
| The calls the agent made: listed newest first with a cursor, shown one
| by one, and decided by a teammate when they wait for the team. Their
| payloads appear only for those who may read payloads.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/agentactions"
	"github.com/dimmihq/dimmi/app/ai/actionkit"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
AIActionCallController serves the agent's calls.
*/
type AIActionCallController struct {
	Calls *agentactions.Calls
	IDs   *ids.Registry
}

/*
AIActionCallsOutput is a page of calls.
*/
type AIActionCallsOutput struct {
	Body fhttp.Collection[resources.AIActionCall]
}

/*
AIActionCallOutput is one call.
*/
type AIActionCallOutput struct {
	Body fhttp.Resource[resources.AIActionCall]
}

/*
Index lists a page of calls.
*/
func (c *AIActionCallController) Index(ctx context.Context, in *requests.AIActionCallsRequest,
) (*AIActionCallsOutput, error) {
	f, err := c.filter(in)
	if err != nil {
		return nil, err
	}
	m := membership(ctx)
	rows, more, err := c.Calls.List(ctx, m, f)
	if err != nil {
		return nil, err
	}
	page := pagination.Page[resources.AIActionCall]{
		Items: resources.NewAIActionCalls(rows, m), Limit: 50,
	}
	if more && len(rows) > 0 {
		page.NextCursor = models.PublicID(models.PrefixAiActionCall, rows[len(rows)-1].ID)
	}
	return &AIActionCallsOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
Show answers one call.
*/
func (c *AIActionCallController) Show(ctx context.Context, in *requests.AIActionCallRequest,
) (*AIActionCallOutput, error) {
	id, err := c.decode(models.PrefixAiActionCall, in.Call)
	if err != nil {
		return nil, err
	}
	m := membership(ctx)
	call, err := c.Calls.Show(ctx, m, id)
	if err != nil {
		return nil, err
	}
	return &AIActionCallOutput{
		Body: fhttp.NewResource(resources.ShowAIActionCall(call, m)),
	}, nil
}

/*
Update approves or declines a call waiting for the team.
*/
func (c *AIActionCallController) Update(ctx context.Context, in *requests.AIUpdateActionCallRequest,
) (*AIActionCallOutput, error) {
	id, err := c.decode(models.PrefixAiActionCall, in.Call)
	if err != nil {
		return nil, err
	}
	m := membership(ctx)
	call, err := c.Calls.Decide(ctx, actionkit.DecideInput{
		Membership: m, CallID: id, Approve: in.Body.Status == "approved", Reason: in.Body.Reason,
		CustomerMessage: in.Body.CustomerMessage,
	})
	if err != nil {
		return nil, err
	}
	return &AIActionCallOutput{
		Body: fhttp.NewResource(resources.ShowAIActionCall(call, m)),
	}, nil
}

/*
filter maps the query to the list's filter.
*/
func (c *AIActionCallController) filter(in *requests.AIActionCallsRequest) (agentactions.CallFilter, error) {
	f := agentactions.CallFilter{}
	if in.State != "" {
		f.State = &in.State
	}
	refs := []struct {
		value  string
		prefix models.Prefix
		dst    **ids.UUID
	}{
		{in.Conversation, models.PrefixConversation, &f.ConversationID},
		{in.Action, models.PrefixAiAction, &f.ActionID},
		{in.Cursor, models.PrefixAiActionCall, &f.Before},
	}
	for _, r := range refs {
		if r.value == "" {
			continue
		}
		id, err := c.decode(r.prefix, r.value)
		if err != nil {
			return f, err
		}
		*r.dst = &id
	}
	return f, nil
}

/*
decode reads a public id of the kind; an unknown one is not found.
*/
func (c *AIActionCallController) decode(prefix models.Prefix, public string) (ids.UUID, error) {
	id, err := c.IDs.Decode(string(prefix), public)
	if err != nil {
		return ids.UUID{}, exceptions.ErrNotFound
	}
	return id, nil
}
