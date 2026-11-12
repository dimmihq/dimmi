/*
|--------------------------------------------------------------------------
| Import Requests
|--------------------------------------------------------------------------
|
| Inputs of the endpoints on imports of customers and companies. The
| file is uploaded as a form with the kind of import; a dry run and a
| start send the same plan: which column fills which field, and whether
| rows matching someone already there update them or are skipped.
|
*/

package requests

import (
	fhttp "github.com/dimmihq/dimmi/framework/http"
)

/*
StoreImportRequest is the input of POST /imports: a form with the file
and the kind, contacts or companies.
*/
type StoreImportRequest struct {
	RawBody fhttp.MultipartForm
}

/*
Kind returns the kind of import the form names.
*/
func (r *StoreImportRequest) Kind() string {
	values := r.RawBody.Value["kind"]
	if len(values) == 0 {
		return ""
	}

	return values[0]
}

/*
ListImportsRequest is the input of GET /imports.
*/
type ListImportsRequest struct {
	Kind string `query:"kind" enum:"contacts,companies" required:"true"`
}

/*
ImportRequest names an import.
*/
type ImportRequest struct {
	Import string `path:"import"`
}

/*
ImportColumn is one column of an import and the field it fills: email,
name, external_id, phone, domain, company_name, company_domain, tags,
or attribute.<key>.
*/
type ImportColumn struct {
	Column int    `json:"column" minimum:"0"`
	Field  string `json:"field"  validate:"required,max=120"`
}

/*
ImportPlanRequest is the input of a dry run or a start of an import.
*/
type ImportPlanRequest struct {
	Import string `path:"import"`
	Body   struct {
		Mapping []ImportColumn `json:"mapping" validate:"required,max=200,dive" maxItems:"200"`
		Mode    string         `json:"mode"    validate:"required,oneof=update skip"`
	}
}
