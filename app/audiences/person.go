/*
|--------------------------------------------------------------------------
| People As Rules See Them
|--------------------------------------------------------------------------
|
| Every surface that asks "does this person match" reads the same
| values: the contact, their company, both sets of tags, what they did,
| and whatever the browser reported about the page. Keeping the lookup in one place
| means a workflow, a preview and "test a person" never disagree about
| what contact.email_domain or company.custom.plan is.
|
*/

package audiences

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/eventroll"
)

/*
Person is one contact with what rules read about them. Tags are public
tag ids. Events are their event summaries by name, read as of Now.
Client holds page, visitor and session values when a browser reported
them; it is empty for a person read from the database alone.
*/
type Person struct {
	Contact     models.Contact
	Company     *models.Company
	ContactTags []string
	CompanyTags []string
	Events      map[string]eventroll.Summary
	Now         time.Time
	Client      map[string]any
}

/*
Lookup returns the value of a contact, company, page, visitor or
session field.
*/
func (p Person) Lookup(key string) (any, bool) {
	root, rest, _ := strings.Cut(key, ".")
	switch root {
	case "contact":
		return Present(ContactValue(p.Contact, p.ContactTags, rest))
	case "company":
		return Present(CompanyValue(p.Company, p.CompanyTags, rest))
	case "event":
		return Present(EventValue(p.Events, rest, p.Now))
	}

	return Present(p.Client[key])
}

/*
ContactValue returns one of a contact's fields by its name after
"contact.", nil when the contact has none.
*/
func ContactValue(c models.Contact, tags []string, name string) any {
	if key, ok := strings.CutPrefix(name, "custom."); ok {
		return CustomValue(c.CustomAttributes, key)
	}

	fields := map[string]any{
		"kind": c.Kind, "email": c.Email, "email_verified": c.EmailVerified, "phone": c.Phone, "name": c.Name,
		"external_id": c.ExternalID, "locale": c.Locale, "timezone": c.Timezone, "created_at": c.CreatedAt,
		"first_seen_at": c.FirstSeenAt, "last_seen_at": c.LastSeenAt, "has_company": c.CompanyID != nil,
		"tags": tags, "email_domain": emailDomain(c.Email),
	}

	if v, ok := fields[name]; ok {
		return v
	}

	return CustomValue(c.Location, name)
}

/*
CompanyValue returns one of a company's fields by its name after
"company.", nil without a company.
*/
func CompanyValue(c *models.Company, tags []string, name string) any {
	if c == nil {
		return nil
	}

	if key, ok := strings.CutPrefix(name, "custom."); ok {
		return CustomValue(c.CustomAttributes, key)
	}

	return map[string]any{"name": c.Name, "domain": c.Domain, "created_at": c.CreatedAt, "tags": tags}[name]
}

/*
Present unwraps pointers and reports whether there is a value.
*/
func Present(v any) (any, bool) {
	switch p := v.(type) {
	case nil:
		return nil, false
	case *string:
		if p == nil {
			return nil, false
		}

		return *p, true
	case *time.Time:
		if p == nil {
			return nil, false
		}

		return *p, true
	case []string:
		return p, true
	}

	return v, true
}

/*
CustomValue reads one key of a JSON object of custom values.
*/
func CustomValue(raw json.RawMessage, key string) any {
	var values map[string]any
	if json.Unmarshal(raw, &values) != nil {
		return nil
	}

	return values[key]
}

/*
emailDomain returns the domain of an address.
*/
func emailDomain(email *string) any {
	if email == nil {
		return nil
	}

	_, domain, ok := strings.Cut(*email, "@")
	if !ok {
		return nil
	}

	return domain
}

/*
LoadPeople reads the companies, tags and event summaries of a page of
contacts, in the contacts' order. Callers set Now before evaluating.
*/
func LoadPeople(ctx context.Context, q *models.Queries, ws ids.UUID, contacts []models.Contact) ([]Person, error) {
	contactIDs := make([]ids.UUID, 0, len(contacts))
	var companyIDs []ids.UUID

	for _, c := range contacts {
		contactIDs = append(contactIDs, c.ID)
		if c.CompanyID != nil {
			companyIDs = append(companyIDs, *c.CompanyID)
		}
	}

	companies, companyTags, err := loadCompanies(ctx, q, ws, companyIDs)
	if err != nil {
		return nil, err
	}

	events, err := LoadEvents(ctx, q, ws, contactIDs)
	if err != nil {
		return nil, err
	}

	tagRows, err := q.ListContactTags(ctx, models.ListContactTagsParams{WorkspaceID: ws, ContactIds: contactIDs})
	if err != nil {
		return nil, fmt.Errorf("list contact tags: %w", err)
	}

	contactTags := map[ids.UUID][]string{}
	for _, r := range tagRows {
		contactTags[r.ContactID] = append(contactTags[r.ContactID], models.PublicID(models.PrefixTag, r.Tag.ID))
	}

	out := make([]Person, 0, len(contacts))
	for _, c := range contacts {
		p := Person{Contact: c, ContactTags: contactTags[c.ID], Events: events[c.ID]}
		if c.CompanyID != nil {
			if company, ok := companies[*c.CompanyID]; ok {
				p.Company, p.CompanyTags = &company, companyTags[company.ID]
			}
		}

		out = append(out, p)
	}

	return out, nil
}

/*
loadCompanies reads companies and their tags by id.
*/
func loadCompanies(ctx context.Context, q *models.Queries, ws ids.UUID, companyIDs []ids.UUID,
) (map[ids.UUID]models.Company, map[ids.UUID][]string, error) {
	companies, tags := map[ids.UUID]models.Company{}, map[ids.UUID][]string{}
	if len(companyIDs) == 0 {
		return companies, tags, nil
	}

	rows, err := q.CompaniesByIDs(ctx, models.CompaniesByIDsParams{WorkspaceID: ws, Ids: companyIDs})
	if err != nil {
		return nil, nil, fmt.Errorf("find companies: %w", err)
	}

	for _, r := range rows {
		companies[r.ID] = r
	}

	tagRows, err := q.ListCompanyTags(ctx, models.ListCompanyTagsParams{WorkspaceID: ws, CompanyIds: companyIDs})
	if err != nil {
		return nil, nil, fmt.Errorf("list company tags: %w", err)
	}

	for _, r := range tagRows {
		tags[r.CompanyID] = append(tags[r.CompanyID], models.PublicID(models.PrefixTag, r.Tag.ID))
	}

	return companies, tags, nil
}
