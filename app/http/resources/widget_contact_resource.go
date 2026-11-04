/*
|--------------------------------------------------------------------------
| Widget Contact Resource
|--------------------------------------------------------------------------
|
| The visitor as their own messenger sees them: who they told the team they
| are, nothing the team wrote about them.
|
*/

package resources

import (
	"encoding/json"

	"github.com/dimmihq/dimmi/app/models"
)

/*
WidgetContact is the visitor as their messenger sees them.
*/
type WidgetContact struct {
	ID     string  `json:"id"`
	Object string  `json:"object"`
	Kind   string  `json:"kind"`
	Name   *string `json:"name"`
	Email  *string `json:"email"`
}

/*
NewWidgetContact returns the visitor's widget-safe profile summary.
*/
func NewWidgetContact(c models.Contact) WidgetContact {
	return WidgetContact{
		ID: models.PublicID(models.PrefixContact, c.ID), Object: "contact", Kind: c.Kind, Name: c.Name, Email: c.Email,
	}
}

/*
WidgetProfile is the visitor's whole profile as their messenger may see it.
*/
type WidgetProfile struct {
	WidgetContact
	EmailVerified    bool            `json:"email_verified"`
	Phone            *string         `json:"phone"`
	Locale           *string         `json:"locale"`
	CustomAttributes json.RawMessage `json:"custom_attributes"`
}

/*
NewWidgetProfile returns the visitor's profile.
*/
func NewWidgetProfile(c models.Contact) WidgetProfile {
	return WidgetProfile{
		WidgetContact: NewWidgetContact(c), EmailVerified: c.EmailVerified, Phone: c.Phone, Locale: c.Locale,
		CustomAttributes: c.CustomAttributes,
	}
}
