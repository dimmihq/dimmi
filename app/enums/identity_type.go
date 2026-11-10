/*
|--------------------------------------------------------------------------
| Identity Type
|--------------------------------------------------------------------------
|
| How a contact appears on a channel: an email address, a phone number,
| a visitor of the messenger. The type is a registry value, so a new
| channel brings its own without a migration.
|
*/

package enums

/*
IdentityType is the kind of a contact identity.
*/
type IdentityType string

/*
The identity types.
*/
const (
	IdentityTypeEmail         IdentityType = "email"
	IdentityTypePhone         IdentityType = "phone"
	IdentityTypeWidgetVisitor IdentityType = "widget_visitor"
	IdentityTypeExternalID    IdentityType = "external_id"
	IdentityTypeSlackUser     IdentityType = "slack_user"
)
