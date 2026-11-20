/*
|--------------------------------------------------------------------------
| Contact Note Controller
|--------------------------------------------------------------------------
|
| The notes the team keeps about a customer or a company: listing them,
| adding one, rewriting or pinning one, and removing one.
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
ContactNoteController serves the notes about customers.
*/
type ContactNoteController struct {
	IDs   *ids.Registry
	Notes *contacts.Notes
}

/*
ContactNotesOutput is a customer's notes.
*/
type ContactNotesOutput struct {
	Body fhttp.Collection[resources.ContactNote]
}

/*
ContactNoteOutput is one note.
*/
type ContactNoteOutput struct {
	Body fhttp.Resource[resources.ContactNote]
}

/*
Index lists a customer's notes, the pinned one first, then newest first.
*/
func (c *ContactNoteController) Index(ctx context.Context, in *requests.ContactNotesRequest,
) (*ContactNotesOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixContact), in.Contact)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	notes, err := c.Notes.List(ctx, membership(ctx), contacts.AboutContact(id))
	if err != nil {
		return nil, err
	}

	items := make([]resources.ContactNote, len(notes))
	for i, n := range notes {
		items[i] = resources.NewContactNote(n, membership(ctx).Member.ID)
	}

	return &ContactNotesOutput{Body: fhttp.NewCollection(pagination.Page[resources.ContactNote]{
		Items: items, Limit: len(items),
	})}, nil
}

/*
Store adds a note about a customer.
*/
func (c *ContactNoteController) Store(ctx context.Context, in *requests.StoreContactNoteRequest,
) (*ContactNoteOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixContact), in.Contact)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	note, err := c.Notes.Create(ctx, membership(ctx), contacts.AboutContact(id), in.Body.Body)
	if err != nil {
		return nil, err
	}

	return &ContactNoteOutput{Body: fhttp.NewResource(resources.NewContactNote(note, membership(ctx).Member.ID))}, nil
}

/*
CompanyIndex lists a company's notes, the pinned one first, then newest
first.
*/
func (c *ContactNoteController) CompanyIndex(ctx context.Context, in *requests.CompanyNotesRequest,
) (*ContactNotesOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixCompany), in.Company)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	notes, err := c.Notes.List(ctx, membership(ctx), contacts.AboutCompany(id))
	if err != nil {
		return nil, err
	}

	items := make([]resources.ContactNote, len(notes))
	for i, n := range notes {
		items[i] = resources.NewContactNote(n, membership(ctx).Member.ID)
	}

	return &ContactNotesOutput{Body: fhttp.NewCollection(pagination.Page[resources.ContactNote]{
		Items: items, Limit: len(items),
	})}, nil
}

/*
CompanyStore adds a note about a company.
*/
func (c *ContactNoteController) CompanyStore(ctx context.Context, in *requests.StoreCompanyNoteRequest,
) (*ContactNoteOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixCompany), in.Company)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	note, err := c.Notes.Create(ctx, membership(ctx), contacts.AboutCompany(id), in.Body.Body)
	if err != nil {
		return nil, err
	}

	return &ContactNoteOutput{Body: fhttp.NewResource(resources.NewContactNote(note, membership(ctx).Member.ID))}, nil
}

/*
Update rewrites a note, pins it or unpins it.
*/
func (c *ContactNoteController) Update(ctx context.Context, in *requests.UpdateContactNoteRequest,
) (*ContactNoteOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixContactNote), in.Note)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	change := contacts.NoteChange{Body: in.Body.Body, Pinned: in.Body.Pinned}

	note, err := c.Notes.Update(ctx, membership(ctx), id, change)
	if err != nil {
		return nil, err
	}

	return &ContactNoteOutput{Body: fhttp.NewResource(resources.NewContactNote(note, membership(ctx).Member.ID))}, nil
}

/*
Destroy removes a note.
*/
func (c *ContactNoteController) Destroy(ctx context.Context, in *requests.ContactNoteRequest) (*struct{}, error) {
	id, err := c.IDs.Decode(string(models.PrefixContactNote), in.Note)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	return &struct{}{}, c.Notes.Delete(ctx, membership(ctx), id)
}
