/*
|--------------------------------------------------------------------------
| Company Resources
|--------------------------------------------------------------------------
|
| A company as the team sees it, with its notes and tags. Contacts and
| the customer panel carry the short form.
|
*/

package resources

import (
	"encoding/json"
	"time"

	"github.com/dimmihq/dimmi/app/actions/companies"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Company is one company. PeopleCount is how many contacts it has, given
only in lists.
*/
type Company struct {
	ID               string          `json:"id"`
	Object           string          `json:"object"`
	ExternalID       *string         `json:"external_id"`
	Name             string          `json:"name"`
	Domain           *string         `json:"domain"`
	CustomAttributes json.RawMessage `json:"custom_attributes"`
	Notes            *string         `json:"notes"`
	Tags             []Tag           `json:"tags"`
	PeopleCount      *int64          `json:"people_count,omitempty"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
}

/*
NewCompany shapes the company with its tags.
*/
func NewCompany(d companies.Detail) Company {
	c := d.Company

	attrs := c.CustomAttributes
	if len(attrs) == 0 {
		attrs = json.RawMessage(`{}`)
	}

	return Company{
		ID: models.PublicID(models.PrefixCompany, c.ID), Object: "company", ExternalID: c.ExternalID, Name: c.Name,
		Domain: c.Domain, CustomAttributes: attrs, Notes: c.Notes, Tags: NewTags(d.Tags), CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
}

/*
NewCompanies shapes a list of companies, in the order given.
*/
func NewCompanies(rows []companies.Detail) []Company {
	out := make([]Company, 0, len(rows))
	for _, d := range rows {
		out = append(out, NewCompany(d))
	}

	return out
}

/*
CompanySummary is the short form of a company.
*/
type CompanySummary struct {
	ID     string  `json:"id"`
	Object string  `json:"object"`
	Name   string  `json:"name"`
	Domain *string `json:"domain"`
}

/*
NewCompanySummary shapes the short form of the company.
*/
func NewCompanySummary(c models.Company) CompanySummary {
	return CompanySummary{
		ID: models.PublicID(models.PrefixCompany, c.ID), Object: "company", Name: c.Name, Domain: c.Domain,
	}
}
