/*
|--------------------------------------------------------------------------
| Ticket Issue Controller
|--------------------------------------------------------------------------
|
| The engineering work behind tickets: the issues linked to a ticket,
| creating or linking one, unlinking, and the drafts that close the
| loop when the work is done.
|
*/

package v1

import (
	"context"
	"strings"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/issues"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
TicketIssueController serves linked issues and drafts.
*/
type TicketIssueController struct {
	TicketController

	Issues *issues.Issues
}

/*
TicketIssuesOutput is a ticket's linked issues.
*/
type TicketIssuesOutput struct {
	Body fhttp.Collection[resources.TicketIssue]
}

/*
Index lists a ticket's linked issues.
*/
func (c *TicketIssueController) Index(ctx context.Context, in *requests.TicketIssuesRequest,
) (*TicketIssuesOutput, error) {
	ref, err := c.ref(in.Ticket)
	if err != nil {
		return nil, err
	}

	list, err := c.Issues.List(ctx, membership(ctx), ref)
	if err != nil {
		return nil, err
	}

	items := resources.NewTicketIssues(list)

	return &TicketIssuesOutput{Body: fhttp.NewCollection(pagination.Page[resources.TicketIssue]{
		Items: items, Limit: len(items),
	})}, nil
}

/*
TicketIssueOutput is one linked issue.
*/
type TicketIssueOutput struct {
	Body fhttp.Resource[resources.TicketIssue]
}

/*
Store creates an issue in a tool, or links one by URL or key.
*/
func (c *TicketIssueController) Store(ctx context.Context, in *requests.StoreTicketIssueRequest,
) (*TicketIssueOutput, error) {
	ref, err := c.ref(in.Ticket)
	if err != nil {
		return nil, err
	}

	b := in.Body
	input := issues.LinkInput{
		Team: b.TeamID, Project: b.ProjectID, Service: b.ServiceID, Severity: b.SeverityID, Title: b.Title,
		Ref: strings.TrimSpace(b.URL + " " + b.Key),
	}

	if b.InstallationID != nil && *b.InstallationID != "" {
		id, err := c.IDs.Decode(string(models.PrefixIntegrationInstallation), *b.InstallationID)
		if err != nil {
			return nil, issues.ErrInstallation
		}

		input.InstallationID = &id
	}

	link, err := c.Issues.Store(ctx, membership(ctx), ref, input)
	if err != nil {
		return nil, err
	}

	return &TicketIssueOutput{Body: fhttp.NewResource(resources.NewTicketIssue(link))}, nil
}

/*
Destroy unlinks an issue.
*/
func (c *TicketIssueController) Destroy(ctx context.Context, in *requests.TicketIssueRequest) (*struct{}, error) {
	id, err := c.IDs.Decode(string(models.PrefixTicketIssue), in.Issue)
	if err != nil {
		return nil, issues.ErrIssueNotFound
	}

	return &struct{}{}, c.Issues.Remove(ctx, membership(ctx), id)
}

/*
TicketUpdateDraftsOutput is a ticket's drafts.
*/
type TicketUpdateDraftsOutput struct {
	Body fhttp.Collection[resources.TicketUpdateDraft]
}

/*
Drafts lists a ticket's drafts with who each would reach.
*/
func (c *TicketIssueController) Drafts(ctx context.Context, in *requests.TicketIssuesRequest,
) (*TicketUpdateDraftsOutput, error) {
	ref, err := c.ref(in.Ticket)
	if err != nil {
		return nil, err
	}

	list, err := c.Issues.Drafts(ctx, membership(ctx), ref)
	if err != nil {
		return nil, err
	}

	items := make([]resources.TicketUpdateDraft, len(list))
	for i, d := range list {
		items[i] = resources.NewTicketUpdateDraft(d)
	}

	return &TicketUpdateDraftsOutput{Body: fhttp.NewCollection(pagination.Page[resources.TicketUpdateDraft]{
		Items: items, Limit: len(items),
	})}, nil
}

/*
TicketUpdateDraftOutput is one draft.
*/
type TicketUpdateDraftOutput struct {
	Body fhttp.Resource[resources.TicketUpdateDraft]
}

/*
UpdateDraft sends or discards a draft.
*/
func (c *TicketIssueController) UpdateDraft(ctx context.Context, in *requests.UpdateTicketUpdateDraftRequest,
) (*TicketUpdateDraftOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixTicketUpdateDraft), in.Draft)
	if err != nil {
		return nil, issues.ErrDraftNotFound
	}

	d, err := c.Issues.Decide(ctx, membership(ctx), id, issues.DecideInput{
		Status: in.Body.Status, Body: in.Body.Body, Resolve: in.Body.Resolve,
	})
	if err != nil {
		return nil, err
	}

	view := resources.NewTicketUpdateDraft(issues.DraftView{Draft: d})

	return &TicketUpdateDraftOutput{Body: fhttp.NewResource(view)}, nil
}

/*
installationID reads an installation's public id.
*/
func installationID(reg *ids.Registry, s string) (ids.UUID, error) {
	id, err := reg.Decode(string(models.PrefixIntegrationInstallation), s)
	if err != nil {
		return ids.Nil(), exceptions.ErrNotFound
	}

	return id, nil
}
