/*
|--------------------------------------------------------------------------
| Contact Removal and Erasure Controllers
|--------------------------------------------------------------------------
|
| DELETE /contacts/{contact} removes a contact with no history (204), or
| with erase=true asks for the customer to be erased (202), immediate=true
| skipping the grace period. GET /contact-erasures lists the requests,
| GET /contact-erasures/{id} shows one, and PATCH with status canceled
| withdraws one during its grace period.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/contacts"
	"github.com/dimmihq/dimmi/app/actions/erasures"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
RemovalOutput answers a removal: no content for a plain delete, the
erasure request for an erasure.
*/
type RemovalOutput struct {
	Status int
	Body   *fhttp.Resource[resources.ContactErasure]
}

/*
ContactErasureOutput is one erasure request.
*/
type ContactErasureOutput struct {
	Body fhttp.Resource[resources.ContactErasure]
}

/*
ContactErasuresOutput is the workspace's erasure requests.
*/
type ContactErasuresOutput struct {
	Body fhttp.Collection[resources.ContactErasure]
}

/*
ErasureController serves the removal and erasure of customers.
*/
type ErasureController struct {
	Remove  *contacts.DeleteContact
	Request *erasures.RequestErasure
	Cancel  *erasures.CancelErasure
	List    *erasures.ListErasures
	Show    *erasures.ShowErasure
	IDs     *ids.Registry
}

/*
Destroy removes the contact, or asks for it to be erased.
*/
func (c *ErasureController) Destroy(ctx context.Context, in *requests.RemoveContactRequest) (*RemovalOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixContact), in.Contact)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	if !in.Erase {
		err := c.Remove.Handle(ctx, contacts.ShowContactInput{Membership: membership(ctx), ContactID: id})

		return &RemovalOutput{Status: fhttp.StatusNoContent}, err
	}

	row, err := c.Request.Handle(ctx, erasures.RequestErasureInput{
		Principal: caller(ctx), Membership: membership(ctx), ContactID: id, Immediate: in.Immediate,
	})
	if err != nil {
		return nil, err
	}

	body := fhttp.NewResource(resources.NewContactErasure(row))

	return &RemovalOutput{Status: fhttp.StatusAccepted, Body: &body}, nil
}

/*
Index lists the workspace's newest erasure requests.
*/
func (c *ErasureController) Index(ctx context.Context, _ *requests.NoInput) (*ContactErasuresOutput, error) {
	rows, err := c.List.Handle(ctx, erasures.ErasureInput{Membership: membership(ctx)})
	if err != nil {
		return nil, err
	}

	page := pagination.Page[resources.ContactErasure]{
		Items: resources.NewContactErasures(rows), Limit: erasures.MaxListed,
	}

	return &ContactErasuresOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
Get shows one erasure request.
*/
func (c *ErasureController) Get(
	ctx context.Context, in *requests.ContactErasureRequest,
) (*ContactErasureOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixContactErasure), in.Erasure)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	row, err := c.Show.Handle(ctx, erasures.ErasureInput{Membership: membership(ctx), ErasureID: id})
	if err != nil {
		return nil, err
	}

	return &ContactErasureOutput{Body: fhttp.NewResource(resources.NewContactErasure(row))}, nil
}

/*
Update withdraws an erasure request and answers with it, listing the
identities it could not give back.
*/
func (c *ErasureController) Update(
	ctx context.Context, in *requests.UpdateContactErasureRequest,
) (*ContactErasureOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixContactErasure), in.Erasure)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	canceled, err := c.Cancel.Handle(ctx, erasures.CancelErasureInput{Membership: membership(ctx), ErasureID: id})
	if err != nil {
		return nil, err
	}

	out := resources.NewContactErasure(canceled.Erasure)
	out.IdentitiesConflicted = canceled.Conflicted

	return &ContactErasureOutput{Body: fhttp.NewResource(out)}, nil
}
