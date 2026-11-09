/*
|--------------------------------------------------------------------------
| Contact Controller
|--------------------------------------------------------------------------
|
| GET /contacts lists the customers of the workspace in play a page at a
| time, GET /contacts/{contact} shows one, POST /contacts adds one, or
| finds it with upsert=true, PATCH /contacts/{contact} changes or blocks
| one and DELETE removes one without history. Blocked customers stay out
| of the list unless asked for. Ids of other workspaces' contacts are
| simply not found.
|
*/

package v1

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/contacts"
	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
maxCustomFilters is the most custom attributes a list may filter by.
*/
const maxCustomFilters = 3

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
ContactStoreOutput is a contact added or found, and which it was.
*/
type ContactStoreOutput struct {
	Status int
	Body   struct {
		Data resources.Contact `json:"data"`
		Meta *UpsertMeta       `json:"meta,omitempty"`
	}
}

/*
ContactController serves the contacts of the workspace in play.
*/
type ContactController struct {
	List          *contacts.ListContacts
	Show          *contacts.ShowContact
	Create        *contacts.CreateContact
	Upsert        *contacts.UpsertIdentifiedContact
	Change        *contacts.UpdateContact
	Conversations *conversations.ListRecordConversations
	IDs           *ids.Registry
	Cursors       *pagination.Cursors
	Clock         clock.Clock
}

/*
contactPage is where a page of contacts starts, carried in its cursor.
*/
type contactPage struct {
	Offset int `json:"o"`
}

/*
contactFilters returns the filters a contact list accepts, with each
custom attribute the query names. Custom attribute names are the
workspace's own, so they cannot be listed up front.
*/
func contactFilters(values url.Values) []string {
	filters := []string{
		"kind", "email", "external_id", "blocked", "company", "tag", "last_seen_after", "last_seen_before",
		"created_after", "created_before",
	}

	for key := range values {
		name, ok := strings.CutPrefix(key, "filter[custom.")
		if ok && strings.HasSuffix(name, "]") {
			filters = append(filters, "custom."+strings.TrimSuffix(name, "]"))
		}
	}

	return filters
}

/*
Index lists a page of contacts, most recently seen first, or by
sort=-created_at or sort=name; the filters and q narrow it.
*/
func (c *ContactController) Index(ctx context.Context, _ *requests.NoInput) (*ContactsOutput, error) {
	r, _ := fhttp.RequestFrom(ctx)

	query, err := pagination.Parse(r.URL.Query(), pagination.Allow{
		Filters: contactFilters(r.URL.Query()),
		Sorts:   []string{contacts.SortLastSeen, contacts.SortCreated, contacts.SortName},
	})
	if err != nil {
		return nil, err
	}

	in, err := c.listInput(ctx, query)
	if err != nil {
		return nil, err
	}

	if in.Conditions, in.Audiences, err = conditionsOf(r.URL.Query().Get("conditions"), c.IDs, c.Clock); err != nil {
		return nil, err
	}

	page, err := c.List.Handle(ctx, in)
	if err != nil {
		return nil, err
	}

	out := pagination.Page[resources.Contact]{
		Items: resources.NewContactDetails(page.Contacts), Limit: in.Limit, Total: page.Total,
	}
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

	blocked, err := blockedFilter(query)
	if err != nil {
		return in, err
	}

	in.Blocked = &blocked

	if err := c.refFilters(query, &in); err != nil {
		return in, err
	}

	if err := timeFilters(query, &in); err != nil {
		return in, err
	}

	if in.Custom, err = customFilter(query); err != nil {
		return in, err
	}

	for _, sort := range query.Sorts {
		in.Sort = sort.Field
	}

	err = c.start(ctx, query, &in)

	return in, err
}

/*
start reads where the page starts: the cursor, or a numbered page within
the counted rows, and whether to count every contact that matches.
*/
func (c *ContactController) start(ctx context.Context, query pagination.Query, in *contacts.ListContactsInput) error {
	offset, count, err := numberedPage(ctx, in.Limit)
	if err != nil {
		return err
	}

	in.Offset, in.Count = offset, count
	if query.Cursor == "" {
		return nil
	}

	var start contactPage
	if err := c.Cursors.Decode(query.Cursor, &start); err != nil {
		return err
	}

	in.Offset = start.Offset

	return nil
}

/*
blockedFilter reads filter[blocked], false by default so blocked
customers stay out of the list unless asked for.
*/
func blockedFilter(query pagination.Query) (bool, error) {
	value := one(query.Filters["blocked"])
	if value == nil {
		return false, nil
	}

	blocked, ok := boolean(*value)
	if !ok {
		return false, pagination.ErrInvalidFilter
	}

	return blocked, nil
}

/*
refFilters reads the filters naming rows.
*/
func (c *ContactController) refFilters(query pagination.Query, in *contacts.ListContactsInput) error {
	var err error

	if in.TagIDs, err = decodeTagFilter(c.IDs, query.Filters["tag"], maxTagFilter); err != nil {
		return err
	}

	if in.CompanyID, err = decodeField(c.IDs, models.PrefixCompany, one(query.Filters["company"]),
		pagination.ErrInvalidFilter); err != nil {
		return err
	}

	return nil
}

/*
timeFilters reads the filters on when a contact was last seen or added.
*/
func timeFilters(query pagination.Query, in *contacts.ListContactsInput) error {
	for name, target := range map[string]**time.Time{
		"last_seen_after": &in.Seen.From, "last_seen_before": &in.Seen.To,
		"created_after": &in.Created.From, "created_before": &in.Created.To,
	} {
		value := one(query.Filters[name])
		if value == nil {
			continue
		}

		at, err := time.Parse(time.RFC3339, *value)
		if err != nil {
			return pagination.ErrInvalidFilter
		}

		*target = &at
	}

	return nil
}

/*
customFilter turns filter[custom.key]=value into a JSON object the
contact's attributes must hold, reading true, false and numbers as what
they look like, since attributes keep their JSON type.
*/
func customFilter(query pagination.Query) (json.RawMessage, error) {
	wanted := map[string]any{}

	for name, values := range query.Filters {
		key, ok := strings.CutPrefix(name, "custom.")
		if !ok || len(values) == 0 {
			continue
		}

		var value any
		if err := json.Unmarshal([]byte(values[0]), &value); err != nil {
			value = values[0]
		}

		wanted[key] = value
	}

	if len(wanted) == 0 {
		return nil, nil
	}

	if len(wanted) > maxCustomFilters {
		return nil, pagination.ErrInvalidFilter
	}

	encoded, err := json.Marshal(wanted)
	if err != nil {
		return nil, pagination.ErrInvalidFilter
	}

	return encoded, nil
}

/*
Get answers with one contact.
*/
func (c *ContactController) Get(ctx context.Context, in *requests.ShowContactRequest) (*ContactOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixContact), in.Contact)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	d, err := c.Show.Handle(ctx, contacts.ShowContactInput{Membership: membership(ctx), ContactID: id})
	if err != nil {
		return nil, err
	}

	return &ContactOutput{Body: fhttp.NewResource(resources.NewContactDetail(d))}, nil
}

/*
Store adds a contact, or with upsert=true finds it by external id, then
email, and changes it.
*/
func (c *ContactController) Store(ctx context.Context, in *requests.StoreContactRequest) (*ContactStoreOutput, error) {
	upsert, err := upsertFlag(in.Upsert)
	if err != nil {
		return nil, err
	}

	if upsert {
		return c.upsert(ctx, in)
	}

	contact, err := c.Create.Handle(ctx, contacts.CreateContactInput{
		Membership: membership(ctx), Fields: contactFields(in.Body.ContactBody),
	})
	if err != nil {
		return nil, err
	}

	out := &ContactStoreOutput{Status: fhttp.StatusCreated}
	out.Body.Data = resources.NewContact(contact)

	return out, nil
}

/*
upsert finds or adds the contact.
*/
func (c *ContactController) upsert(ctx context.Context, in *requests.StoreContactRequest) (*ContactStoreOutput, error) {
	input := contacts.UpsertInput{
		Membership: membership(ctx), Fields: contactFields(in.Body.ContactBody), Cleared: requests.Nulls(in.RawBody),
	}

	if in.Body.Company != nil {
		input.Company = new(companyFields(*in.Body.Company))
	}

	upserted, err := c.Upsert.Handle(ctx, input)
	if err != nil {
		return nil, err
	}

	out := &ContactStoreOutput{Status: fhttp.StatusCreated}
	out.Body.Data = resources.NewContactDetail(upserted.Detail)
	out.Body.Meta = &UpsertMeta{Upserted: "created"}

	if !upserted.Created {
		out.Status, out.Body.Meta.Upserted = fhttp.StatusOK, "updated"
	}

	for _, lead := range upserted.PossibleDuplicates {
		out.Body.Meta.PossibleDuplicates = append(out.Body.Meta.PossibleDuplicates, resources.NewContactSummary(lead))
	}

	return out, nil
}

/*
Update changes a contact and answers with it as it now is.
*/
func (c *ContactController) Update(ctx context.Context, in *requests.UpdateContactRequest) (*ContactOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixContact), in.Contact)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	nulls, sent := requests.Nulls(in.RawBody), requests.Sent(in.RawBody)

	company, err := decodeField(c.IDs, models.PrefixCompany, in.Body.CompanyID, exceptions.ErrCompanyNotFound)
	if err != nil {
		return nil, err
	}

	d, err := c.Change.Handle(ctx, contacts.UpdateContactInput{
		Membership: membership(ctx), ContactID: id, Fields: contactFields(in.Body.ContactBody),
		ClearExternalID: nulls["external_id"], ClearEmail: nulls["email"], ClearName: nulls["name"],
		ClearPhone: nulls["phone"], Notes: in.Body.Notes, ClearNotes: nulls["notes"], CompanySet: sent["company_id"],
		CompanyID: company, AddTags: tagRefs(c.IDs, in.Body.AddTags),
		RemoveTags: decodeTagIDs(c.IDs, in.Body.RemoveTags), Blocked: in.Body.Blocked,
	})
	if err != nil {
		return nil, err
	}

	return &ContactOutput{Body: fhttp.NewResource(resources.NewContactDetail(d))}, nil
}

/*
ConversationsIndex lists a page of the contact's conversations, newest
first, as far as the reader may see.
*/
func (c *ContactController) ConversationsIndex(
	ctx context.Context, in *requests.ShowContactRequest,
) (*ConversationsOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixContact), in.Contact)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	return recordConversations(ctx, c.Conversations, c.Cursors, conversations.RecordConversationsInput{
		Membership: membership(ctx), ContactID: &id,
	})
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
