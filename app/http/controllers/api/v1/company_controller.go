/*
|--------------------------------------------------------------------------
| Company Controller
|--------------------------------------------------------------------------
|
| GET /companies lists the workspace's companies a page at a time, GET
| /companies/{company} shows one, POST /companies adds one or finds it
| with upsert=true, PATCH changes one and DELETE removes it. GET
| .../contacts lists its people and GET .../conversations what they have
| written about, as far as the reader may see. Ids of other workspaces'
| companies are simply not found.
|
*/

package v1

import (
	"context"
	"time"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/companies"
	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/querybuilder"
)

/*
CompaniesOutput is a page of companies.
*/
type CompaniesOutput struct {
	Body fhttp.Collection[resources.Company]
}

/*
CompanyOutput is one company.
*/
type CompanyOutput struct {
	Body fhttp.Resource[resources.Company]
}

/*
CompanyStoreOutput is a company added or found, and which it was.
*/
type CompanyStoreOutput struct {
	Status int
	Body   struct {
		Data resources.Company `json:"data"`
		Meta *UpsertMeta       `json:"meta,omitempty"`
	}
}

/*
UpsertMeta says whether an upsert added the record or changed it.
*/
type UpsertMeta struct {
	Upserted           string                     `json:"upserted"`
	PossibleDuplicates []resources.ContactSummary `json:"possible_duplicates,omitempty"`
}

/*
CompanyController serves the companies of the workspace in play.
*/
type CompanyController struct {
	List          *companies.ListCompanies
	Show          *companies.ShowCompany
	Create        *companies.CreateCompany
	Identify      *companies.IdentifyCompany
	Change        *companies.UpdateCompany
	Remove        *companies.DeleteCompany
	People        *companies.ListCompanyContacts
	Conversations *conversations.ListRecordConversations
	IDs           *ids.Registry
	Cursors       *pagination.Cursors
}

/*
timeCursor is where a page ordered by creation time starts.
*/
type timeCursor struct {
	At time.Time `json:"t"`
	ID ids.UUID  `json:"i"`
}

/*
CompanyListing is what the company list accepts: a filter on tags.
*/
func CompanyListing() pagination.Allow {
	return pagination.Allow{Filters: []string{"tag"}}
}

/*
Index lists a page of companies, newest first; filter[tag] and q narrow
it.
*/
func (c *CompanyController) Index(ctx context.Context, _ *requests.NoInput) (*CompaniesOutput, error) {
	r, _ := fhttp.RequestFrom(ctx)

	query, err := pagination.Parse(r.URL.Query(), CompanyListing())
	if err != nil {
		return nil, err
	}

	in := companies.ListInput{Membership: membership(ctx), Limit: query.Limit}
	if in.Offset, in.Count, err = numberedPage(ctx, in.Limit); err != nil {
		return nil, err
	}

	if query.Search != "" {
		in.Search = &query.Search
	}

	if in.TagIDs, err = decodeTagFilter(c.IDs, query.Filters["tag"], maxTagFilter); err != nil {
		return nil, err
	}

	if query.Cursor != "" {
		var start timeCursor
		if err := c.Cursors.Decode(query.Cursor, &start); err != nil {
			return nil, err
		}

		in.AfterAt, in.AfterID = &start.At, &start.ID
	}

	page, err := c.List.Handle(ctx, in)
	if err != nil {
		return nil, err
	}

	items := resources.NewCompanies(page.Companies)
	for i, d := range page.Companies {
		items[i].PeopleCount = new(page.People[d.Company.ID])
	}

	out := pagination.Page[resources.Company]{Items: items, Limit: in.Limit, Total: page.Total}
	if page.More {
		last := page.Companies[len(page.Companies)-1].Company
		if out.NextCursor, err = c.Cursors.Encode(timeCursor{At: last.CreatedAt, ID: last.ID}); err != nil {
			return nil, err
		}
	}

	return &CompaniesOutput{Body: fhttp.NewCollection(out)}, nil
}

/*
Get answers with one company.
*/
func (c *CompanyController) Get(ctx context.Context, in *requests.CompanyRequest) (*CompanyOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixCompany), in.Company)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	d, err := c.Show.Handle(ctx, companies.ShowInput{Membership: membership(ctx), CompanyID: id})
	if err != nil {
		return nil, err
	}

	return &CompanyOutput{Body: fhttp.NewResource(resources.NewCompany(d))}, nil
}

/*
Store adds a company, or with upsert=true finds it by external id, then
domain, and changes it.
*/
func (c *CompanyController) Store(ctx context.Context, in *requests.StoreCompanyRequest) (*CompanyStoreOutput, error) {
	upsert, err := upsertFlag(in.Upsert)
	if err != nil {
		return nil, err
	}

	out := &CompanyStoreOutput{Status: fhttp.StatusCreated}
	fields := companyFields(in.Body)

	if !upsert {
		if fields.Name == nil {
			return nil, exceptions.ErrCompanyNameRequired
		}

		company, err := c.Create.Handle(ctx, companies.CreateInput{Membership: membership(ctx), Fields: fields})
		if err != nil {
			return nil, err
		}

		out.Body.Data = resources.NewCompany(companies.Detail{Company: company})

		return out, nil
	}

	d, created, err := c.Identify.Handle(ctx, companies.IdentifyInput{Membership: membership(ctx), Fields: fields})
	if err != nil {
		return nil, err
	}

	out.Body.Data, out.Body.Meta = resources.NewCompany(d), &UpsertMeta{Upserted: "created"}
	if !created {
		out.Status, out.Body.Meta.Upserted = fhttp.StatusOK, "updated"
	}

	return out, nil
}

/*
Update changes a company and answers with it as it now is.
*/
func (c *CompanyController) Update(ctx context.Context, in *requests.UpdateCompanyRequest) (*CompanyOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixCompany), in.Company)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	d, err := c.Change.Handle(ctx, companies.UpdateInput{
		Membership: membership(ctx), CompanyID: id, Fields: companyFields(in.Body.CompanyBody),
		Cleared: requests.Nulls(in.RawBody), Notes: in.Body.Notes, AddTags: tagRefs(c.IDs, in.Body.AddTags),
		RemoveTags: decodeTagIDs(c.IDs, in.Body.RemoveTags),
	})
	if err != nil {
		return nil, err
	}

	return &CompanyOutput{Body: fhttp.NewResource(resources.NewCompany(d))}, nil
}

/*
Destroy deletes a company.
*/
func (c *CompanyController) Destroy(ctx context.Context, in *requests.CompanyRequest) (*EmptyOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixCompany), in.Company)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	if err := c.Remove.Handle(ctx, companies.ShowInput{Membership: membership(ctx), CompanyID: id}); err != nil {
		return nil, err
	}

	return &EmptyOutput{}, nil
}

/*
Contacts lists a page of the company's people, newest first.
*/
func (c *CompanyController) Contacts(ctx context.Context, in *requests.CompanyRequest) (*ContactsOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixCompany), in.Company)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	r, _ := fhttp.RequestFrom(ctx)

	query, err := pagination.Parse(r.URL.Query(), PageListing())
	if err != nil {
		return nil, err
	}

	input := companies.ContactsInput{Membership: membership(ctx), CompanyID: id, Limit: query.Limit}

	if query.Cursor != "" {
		var start timeCursor
		if err := c.Cursors.Decode(query.Cursor, &start); err != nil {
			return nil, err
		}

		input.AfterAt, input.AfterID = &start.At, &start.ID
	}

	page, err := c.People.Handle(ctx, input)
	if err != nil {
		return nil, err
	}

	out := pagination.Page[resources.Contact]{Items: resources.NewContacts(page.Contacts), Limit: input.Limit}
	if page.More {
		last := page.Contacts[len(page.Contacts)-1]
		if out.NextCursor, err = c.Cursors.Encode(timeCursor{At: last.CreatedAt, ID: last.ID}); err != nil {
			return nil, err
		}
	}

	return &ContactsOutput{Body: fhttp.NewCollection(out)}, nil
}

/*
ConversationsIndex lists a page of the conversations of the company's
people, newest first.
*/
func (c *CompanyController) ConversationsIndex(
	ctx context.Context, in *requests.CompanyRequest,
) (*ConversationsOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixCompany), in.Company)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	return recordConversations(ctx, c.Conversations, c.Cursors, conversations.RecordConversationsInput{
		Membership: membership(ctx), CompanyID: &id,
	})
}

/*
companyFields turns the request's details into the action's.
*/
func companyFields(b requests.CompanyBody) companies.Fields {
	return companies.Fields{
		ExternalID: b.ExternalID, Name: b.Name, Domain: b.Domain, CustomAttributes: b.CustomAttributes,
	}
}

/*
upsertFlag reads the upsert parameter, which is true, false or absent.
*/
func upsertFlag(value string) (bool, error) {
	if value == "" {
		return false, nil
	}

	flag, ok := boolean(value)
	if !ok {
		return false, exceptions.ErrInvalidUpsertFlag
	}

	return flag, nil
}

/*
recordConversations answers with a page of a record's conversations, the
cursor and limit read from the request.
*/
func recordConversations(
	ctx context.Context, list *conversations.ListRecordConversations, cursors *pagination.Cursors,
	in conversations.RecordConversationsInput,
) (*ConversationsOutput, error) {
	r, _ := fhttp.RequestFrom(ctx)

	query, err := pagination.Parse(r.URL.Query(), PageListing())
	if err != nil {
		return nil, err
	}

	in.Limit = query.Limit

	if query.Cursor != "" {
		var start querybuilder.Keyset
		if err := cursors.Decode(query.Cursor, &start); err != nil {
			return nil, err
		}

		in.After = &start
	}

	page, err := list.Handle(ctx, in)
	if err != nil {
		return nil, err
	}

	out := pagination.Page[resources.Conversation]{
		Items: resources.NewConversations(page.Conversations), Limit: in.Limit,
	}
	if page.More && page.Last != nil {
		if out.NextCursor, err = cursors.Encode(page.Last); err != nil {
			return nil, err
		}
	}

	return &ConversationsOutput{Body: fhttp.NewCollection(out)}, nil
}
