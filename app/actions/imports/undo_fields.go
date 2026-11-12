/*
|--------------------------------------------------------------------------
| Undoing Fields
|--------------------------------------------------------------------------
|
| The fields an undo puts back become the change a teammate would send:
| a field that was empty before is cleared, any other gets its old
| value, and a custom attribute that did not exist before is removed.
|
*/

package imports

import (
	"strings"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/companies"
	"github.com/dimmihq/dimmi/app/actions/contacts"
)

/*
contactRevert turns the fields to put back into a change of a customer.
*/
func contactRevert(back snapshot) contacts.UpdateContactInput {
	in := contacts.UpdateContactInput{Fields: contacts.Fields{CustomAttributes: map[string]any{}}}

	for k, v := range back {
		text, _ := v.(string)

		switch k {
		case "email":
			in.Fields.Email, in.ClearEmail = textOf(text), text == ""
		case "name":
			in.Fields.Name, in.ClearName = textOf(text), text == ""
		case "phone":
			in.Fields.Phone, in.ClearPhone = textOf(text), text == ""
		case "company_id":
			in.CompanySet = true
			if id, err := ids.Parse(text); err == nil {
				in.CompanyID = &id
			}
		default:
			if key, ok := strings.CutPrefix(k, attributePrefix); ok {
				in.Fields.CustomAttributes[key] = v
			}
		}
	}

	return in
}

/*
companyRevert turns the fields to put back into a change of a company.
*/
func companyRevert(back snapshot) companies.UpdateInput {
	in := companies.UpdateInput{
		Fields: companies.Fields{CustomAttributes: map[string]any{}}, Cleared: map[string]bool{},
	}

	for k, v := range back {
		text, _ := v.(string)

		switch k {
		case "name":
			in.Fields.Name = textOf(text)
		case "domain":
			in.Fields.Domain, in.Cleared["domain"] = textOf(text), text == ""
		default:
			if key, ok := strings.CutPrefix(k, attributePrefix); ok {
				in.Fields.CustomAttributes[key] = v
			}
		}
	}

	return in
}

/*
textOf returns the text, nil when empty.
*/
func textOf(s string) *string {
	if s == "" {
		return nil
	}

	return &s
}
