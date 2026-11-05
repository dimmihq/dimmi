/*
|--------------------------------------------------------------------------
| Company Requests
|--------------------------------------------------------------------------
|
| Inputs of the endpoints on companies. An update keeps the raw body to
| tell a field cleared with null from one left out.
|
*/

package requests

/*
CompanyRequest is the input of the endpoints that name only a company.
*/
type CompanyRequest struct {
	Company string `path:"company"`
}

/*
CompanyBody is a company's details as a request sends them.
*/
type CompanyBody struct {
	ExternalID       *string        `json:"external_id,omitempty"       validate:"omitempty,min=1,max=255"`
	Name             *string        `json:"name,omitempty"              validate:"omitempty,min=1,max=255"`
	Domain           *string        `json:"domain,omitempty"            validate:"omitempty,max=255"`
	CustomAttributes map[string]any `json:"custom_attributes,omitempty"`
}

/*
StoreCompanyRequest is the input of POST /companies: a plain create, or
with upsert=true a find-or-add by external id, then domain.
*/
type StoreCompanyRequest struct {
	Upsert string `query:"upsert"`
	Body   CompanyBody
}

/*
UpdateCompanyRequest is the input of PATCH /companies/{company}.
*/
type UpdateCompanyRequest struct {
	Company string `path:"company"`
	Body    struct {
		CompanyBody

		Notes      *string  `json:"notes,omitempty"       validate:"omitempty,max=10000"`
		AddTags    []string `json:"add_tags,omitempty"    validate:"omitempty,max=50,dive,min=1,max=100"`
		RemoveTags []string `json:"remove_tags,omitempty" validate:"omitempty,max=50"`
	}
	RawBody []byte
}
