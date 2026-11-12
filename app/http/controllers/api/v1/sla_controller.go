/*
|--------------------------------------------------------------------------
| SLA Controller
|--------------------------------------------------------------------------
|
| GET /sla-policies lists the workspace's service level policies,
| archived ones last; POST adds one; GET, PATCH and DELETE read, change
| and archive one. Archiving cancels the clocks still running. GET
| /tickets/{ticket}/sla-clocks and /conversations/{conversation}/
| sla-clocks list every clock of a ticket or a conversation, newest
| first.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/actions/sla"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
SLAController serves service level policies and clocks. Tickets reads
ticket references.
*/
type SLAController struct {
	SLA     *sla.SLA
	IDs     *ids.Registry
	Tickets *TicketController
}

/*
SLAPoliciesOutput is the workspace's policies.
*/
type SLAPoliciesOutput struct {
	Body fhttp.Collection[resources.SLAPolicy]
}

/*
SLAPolicyOutput is one policy.
*/
type SLAPolicyOutput struct {
	Body fhttp.Resource[resources.SLAPolicy]
}

/*
SLAClocksOutput is the clocks of a ticket or a conversation.
*/
type SLAClocksOutput struct {
	Body fhttp.Collection[resources.SLAClock]
}

/*
Index lists the policies.
*/
func (c *SLAController) Index(ctx context.Context, _ *requests.NoInput) (*SLAPoliciesOutput, error) {
	rows, err := c.SLA.List(ctx, membership(ctx))
	if err != nil {
		return nil, err
	}

	page := pagination.Page[resources.SLAPolicy]{Items: resources.NewSLAPolicies(rows), Limit: 100}

	return &SLAPoliciesOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
Show answers one policy.
*/
func (c *SLAController) Show(ctx context.Context, in *requests.SLAPolicyRequest) (*SLAPolicyOutput, error) {
	id, err := c.decode(models.PrefixSLAPolicy, in.Policy)
	if err != nil {
		return nil, err
	}

	p, err := c.SLA.Show(ctx, membership(ctx), id)

	return policyOutput(p, err)
}

/*
Store adds a policy.
*/
func (c *SLAController) Store(ctx context.Context, in *requests.StoreSLAPolicyRequest) (*SLAPolicyOutput, error) {
	input, err := c.input(in.Body)
	if err != nil {
		return nil, err
	}

	p, err := c.SLA.Create(ctx, membership(ctx), input)

	return policyOutput(p, err)
}

/*
Update changes a policy.
*/
func (c *SLAController) Update(ctx context.Context, in *requests.UpdateSLAPolicyRequest) (*SLAPolicyOutput, error) {
	id, err := c.decode(models.PrefixSLAPolicy, in.Policy)
	if err != nil {
		return nil, err
	}

	input, err := c.input(in.Body)
	if err != nil {
		return nil, err
	}

	p, err := c.SLA.Update(ctx, membership(ctx), id, input)

	return policyOutput(p, err)
}

/*
Destroy archives a policy.
*/
func (c *SLAController) Destroy(ctx context.Context, in *requests.SLAPolicyRequest) (*EmptyOutput, error) {
	id, err := c.decode(models.PrefixSLAPolicy, in.Policy)
	if err != nil {
		return nil, err
	}

	return &EmptyOutput{}, c.SLA.Archive(ctx, membership(ctx), id)
}

/*
TicketClocks lists a ticket's clocks.
*/
func (c *SLAController) TicketClocks(ctx context.Context, in *requests.TicketRequest) (*SLAClocksOutput, error) {
	ref, err := c.Tickets.ref(in.Ticket)
	if err != nil {
		return nil, err
	}

	rows, err := c.SLA.TicketClocks(ctx, membership(ctx), ref)

	return clocksOutput(rows, err)
}

/*
ConversationClocks lists a conversation's clocks and its ticket's.
*/
func (c *SLAController) ConversationClocks(
	ctx context.Context, in *requests.ConversationSLAClocksRequest,
) (*SLAClocksOutput, error) {
	id, err := c.decode(models.PrefixConversation, in.Conversation)
	if err != nil {
		return nil, err
	}

	rows, err := c.SLA.ConversationClocks(ctx, membership(ctx), conversations.Ref{ID: id})

	return clocksOutput(rows, err)
}

func policyOutput(p models.SlaPolicy, err error) (*SLAPolicyOutput, error) {
	if err != nil {
		return nil, err
	}

	return &SLAPolicyOutput{Body: fhttp.NewResource(resources.NewSLAPolicy(p))}, nil
}

func clocksOutput(rows []models.SlaClock, err error) (*SLAClocksOutput, error) {
	if err != nil {
		return nil, err
	}

	page := pagination.Page[resources.SLAClock]{Items: resources.NewSLAClocks(rows), Limit: 100}

	return &SLAClocksOutput{Body: fhttp.NewCollection(page)}, nil
}

func (c *SLAController) decode(prefix models.Prefix, public string) (ids.UUID, error) {
	id, err := c.IDs.Decode(string(prefix), public)
	if err != nil {
		return ids.UUID{}, exceptions.ErrNotFound
	}

	return id, nil
}
