/*
|--------------------------------------------------------------------------
| Contact Controller
|--------------------------------------------------------------------------
|
| GET /contacts lists the customers of the workspace in play a page at a
| time, GET /contacts/{contact} shows one, POST /contacts adds one and
| PATCH /contacts/{contact} changes or blocks one. Ids of other
| workspaces' contacts are simply not found.
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
ContactsOutput is a page of contacts.
*/
type ContactsOutput struct {
	Body fhttp.Collection[resources.Contact]
}

/*
ContactOutput is one contact.
*/
type ContactOutput struct {
	Body fhttp.Resource[resources.Contact]
}

/*
ContactController serves the contacts of the workspace in play.
*/
type ContactController struct {
	List    *contacts.ListContacts
	Show    *contacts.ShowContact
	Create  *contacts.CreateContact
	Change  *contacts.UpdateContact
	IDs     *ids.Registry
	Cursors *pagination.Cursors
}

/*
contactPage is where a page of contacts starts, carried in its cursor.
*/
type contactPage struct {
	Offset int `json:"o"`
}

/*
Index lists a page of contacts, most recently seen first, or by
sort=-created_at or sort=name; filter[kind], filter[email],
filter[external_id], filter[blocked] and q narrow it.
*/
func (c *ContactController) Index(ctx context.Context, _ *requests.NoInput) (*ContactsOutput, error) {
	r, _ := fhttp.RequestFrom(ctx)

	query, err := pagination.Parse(r.URL.Query(), pagination.Allow{
		Filters: []string{"kind", "email", "external_id", "blocked"},
		Sorts:   []string{contacts.SortLastSeen, contacts.SortCreated, contacts.SortName},
	})
	if err != nil {
		return nil, err
	}

	in, err := c.listInput(ctx, query)
	if err != nil {
		return nil, err
	}

	page, err := c.List.Handle(ctx, in)
	if err != nil {
		return nil, err
	}

	out := pagination.Page[resources.Contact]{Items: resources.NewContacts(page.Contacts), Limit: in.Limit}
	if page.More {
		if out.NextCursor, err = c.Cursors.Encode(contactPage{Offset: in.Offset + in.Limit}); err != nil {
			return nil, err
		}
	}

	return &ContactsOutput{Body: fhttp.NewCollection(out)}, nil
}

/*
listInput turns the list query into the action's input.
*/
func (c *ContactController) listInput(ctx context.Context, query pagination.Query) (contacts.ListContactsInput, error) {
	in := contacts.ListContactsInput{
		Membership: membership(ctx), Search: query.Search, Limit: query.Limit, Sort: contacts.SortLastSeen,
		Kind: one(query.Filters["kind"]), Email: one(query.Filters["email"]),
		ExternalID: one(query.Filters["external_id"]),
	}

	if kind := in.Kind; kind != nil && *kind != "user" && *kind != "lead" {
		return in, pagination.ErrInvalidFilter
	}

	if blocked := one(query.Filters["blocked"]); blocked != nil {
		value, ok := boolean(*blocked)
		if !ok {
			return in, pagination.ErrInvalidFilter
		}

		in.Blocked = &value
	}

	for _, sort := range query.Sorts {
		in.Sort = sort.Field
	}

	if query.Cursor != "" {
		var start contactPage
		if err := c.Cursors.Decode(query.Cursor, &start); err != nil {
			return in, err
		}

		in.Offset = start.Offset
	}

	return in, nil
}

/*
Get answers with one contact.
*/
func (c *ContactController) Get(ctx context.Context, in *requests.ShowContactRequest) (*ContactOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixContact), in.Contact)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	contact, err := c.Show.Handle(ctx, contacts.ShowContactInput{Membership: membership(ctx), ContactID: id})
	if err != nil {
		return nil, err
	}

	return &ContactOutput{Body: fhttp.NewResource(resources.NewContact(contact))}, nil
}

/*
Store adds a contact and answers with it.
*/
func (c *ContactController) Store(ctx context.Context, in *requests.StoreContactRequest) (*ContactOutput, error) {
	contact, err := c.Create.Handle(ctx, contacts.CreateContactInput{
		Membership: membership(ctx), Fields: contactFields(in.Body),
	})
	if err != nil {
		return nil, err
	}

	return &ContactOutput{Body: fhttp.NewResource(resources.NewContact(contact))}, nil
}

/*
Update changes a contact and answers with it as it now is.
*/
func (c *ContactController) Update(ctx context.Context, in *requests.UpdateContactRequest) (*ContactOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixContact), in.Contact)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	nulls := requests.Nulls(in.RawBody)

	contact, err := c.Change.Handle(ctx, contacts.UpdateContactInput{
		Membership: membership(ctx), ContactID: id, Fields: contactFields(in.Body.ContactBody),
		ClearExternalID: nulls["external_id"], ClearEmail: nulls["email"], ClearName: nulls["name"],
		ClearPhone: nulls["phone"], Blocked: in.Body.Blocked,
	})
	if err != nil {
		return nil, err
	}

	return &ContactOutput{Body: fhttp.NewResource(resources.NewContact(contact))}, nil
}

/*
contactFields turns the request's details into the action's.
*/
func contactFields(b requests.ContactBody) contacts.Fields {
	return contacts.Fields{
		ExternalID: b.ExternalID, Email: b.Email, Name: b.Name, Phone: b.Phone, CustomAttributes: b.CustomAttributes,
	}
}

/*
one returns the only value of a filter, nil when it has none.
*/
func one(values []string) *string {
	if len(values) == 0 {
		return nil
	}

	return &values[0]
}

/*
boolean reads true or false.
*/
func boolean(s string) (bool, bool) {
	switch s {
	case "true":
		return true, true
	case "false":
		return false, true
	}

	return false, false
}
