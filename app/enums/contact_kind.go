/*
|--------------------------------------------------------------------------
| Contact Kind
|--------------------------------------------------------------------------
|
| A user is a contact the host app identified, through a verified token
| or the API. Everyone else is a lead, and nothing a lead says about
| themselves is trusted.
|
*/

package enums

/*
ContactKind says whether a contact was identified by the host app.
*/
type ContactKind string

/*
The contact kinds.
*/
const (
	ContactKindUser ContactKind = "user"
	ContactKindLead ContactKind = "lead"
)

/*
ContactKinds returns every kind, in the order of the database check.
*/
func ContactKinds() []ContactKind {
	return []ContactKind{ContactKindUser, ContactKindLead}
}

/*
Valid reports whether the kind is one of the kinds.
*/
func (k ContactKind) Valid() bool {
	switch k {
	case ContactKindUser, ContactKindLead:
		return true
	}

	return false
}
