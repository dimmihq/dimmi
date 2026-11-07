/*
|--------------------------------------------------------------------------
| Identity Token Check Resource
|--------------------------------------------------------------------------
|
| What the token checker made of a pasted token. The header and claims
| go back only to the member who pasted them, never the signature.
|
*/

package resources

import (
	"github.com/dimmihq/dimmi/app/actions/identity"
	"github.com/dimmihq/dimmi/app/models"
)

/*
IdentityMistake is a recognized slip and its fix.
*/
type IdentityMistake struct {
	Code string `json:"code"`
	Hint string `json:"hint"`
}

/*
IdentityContactPreview says whether a valid token would reuse a known
customer or create one.
*/
type IdentityContactPreview struct {
	Action    string  `json:"action"`
	ContactID *string `json:"contact_id"`
}

/*
IdentityValidation is what the token checker found.
*/
type IdentityValidation struct {
	Object         string                  `json:"object"`
	Valid          bool                    `json:"valid"`
	Code           *string                 `json:"code"`
	Message        *string                 `json:"message"`
	Hint           *string                 `json:"hint"`
	Mistake        *IdentityMistake        `json:"mistake"`
	Header         map[string]any          `json:"header"`
	Claims         map[string]any          `json:"claims"`
	ContactPreview *IdentityContactPreview `json:"contact_preview"`
	Warnings       []IdentityMistake       `json:"warnings"`
}

/*
NewIdentityValidation shapes a check.
*/
func NewIdentityValidation(v identity.Validation) IdentityValidation {
	out := IdentityValidation{
		Object: "identity_validation", Valid: v.Valid, Code: optional(v.Code), Message: optional(v.Message),
		Hint: optional(v.Hint), Header: v.Header, Claims: v.Claims, Warnings: make([]IdentityMistake, len(v.Warnings)),
	}
	if v.Mistake != nil {
		out.Mistake = &IdentityMistake{Code: v.Mistake.Code, Hint: v.Mistake.Hint}
	}
	for i, w := range v.Warnings {
		out.Warnings[i] = IdentityMistake{Code: w.Code, Hint: w.Hint}
	}
	if v.Preview != nil {
		out.ContactPreview = &IdentityContactPreview{Action: v.Preview.Action}
		if v.Preview.ContactID != nil {
			out.ContactPreview.ContactID = new(models.PublicID(models.PrefixContact, *v.Preview.ContactID))
		}
	}
	return out
}
