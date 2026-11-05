/*
|--------------------------------------------------------------------------
| Contact Identity And Duplicate Controllers
|--------------------------------------------------------------------------
|
| GET /contacts/{contact}/identities lists how a customer appears on
| channels and DELETE detaches one attached by mistake. GET
| /contacts/{contact}/duplicates lists who looks like the same person,
| with the reasons, for a teammate to judge.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/contacts"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
The most rows the unpaged lists on a contact return.
*/
const (
	maxIdentityPage  = 200
	maxDuplicatePage = 20
)

/*
ContactIdentitiesOutput is how a contact appears on channels.
*/
type ContactIdentitiesOutput struct {
	Body fhttp.Collection[resources.ContactIdentity]
}

/*
ContactDuplicatesOutput is who looks like the same person as a contact.
*/
type ContactDuplicatesOutput struct {
	Body fhttp.Collection[resources.ContactDuplicate]
}

/*
ContactIdentityController serves a contact's identities and possible
duplicates.
*/
type ContactIdentityController struct {
	List       *contacts.ListIdentities
	Detach     *contacts.DetachIdentity
	Duplicates *contacts.FindDuplicateContacts
	IDs        *ids.Registry
}

/*
Index lists the contact's identities.
*/
func (c *ContactIdentityController) Index(
	ctx context.Context, in *requests.ShowContactRequest,
) (*ContactIdentitiesOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixContact), in.Contact)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	rows, err := c.List.Handle(ctx, contacts.ShowContactInput{Membership: membership(ctx), ContactID: id})
	if err != nil {
		return nil, err
	}

	page := pagination.Page[resources.ContactIdentity]{
		Items: resources.NewContactIdentities(rows), Limit: maxIdentityPage,
	}

	return &ContactIdentitiesOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
Destroy detaches an identity from the contact.
*/
func (c *ContactIdentityController) Destroy(
	ctx context.Context, in *requests.ContactIdentityRequest,
) (*EmptyOutput, error) {
	contact, err := c.IDs.Decode(string(models.PrefixContact), in.Contact)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	identity, err := c.IDs.Decode(string(models.PrefixContactIdentity), in.Identity)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	err = c.Detach.Handle(ctx, contacts.DetachIdentityInput{
		Membership: membership(ctx), ContactID: contact, IdentityID: identity,
	})
	if err != nil {
		return nil, err
	}

	return &EmptyOutput{}, nil
}

/*
DuplicatesIndex lists who looks like the same person, and why.
*/
func (c *ContactIdentityController) DuplicatesIndex(
	ctx context.Context, in *requests.ShowContactRequest,
) (*ContactDuplicatesOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixContact), in.Contact)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	rows, err := c.Duplicates.Handle(ctx, contacts.ShowContactInput{Membership: membership(ctx), ContactID: id})
	if err != nil {
		return nil, err
	}

	page := pagination.Page[resources.ContactDuplicate]{
		Items: resources.NewContactDuplicates(rows), Limit: maxDuplicatePage,
	}

	return &ContactDuplicatesOutput{Body: fhttp.NewCollection(page)}, nil
}
