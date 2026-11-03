/*
|--------------------------------------------------------------------------
| Author Type
|--------------------------------------------------------------------------
|
| Who wrote a timeline entry decides what its author id points at: a
| contact, a member, an AI agent, an integration, or nothing at all for
| the system itself.
|
*/

package enums

/*
AuthorType is the kind of author of a timeline entry.
*/
type AuthorType string

/*
The author types.
*/
const (
	AuthorTypeContact     AuthorType = "contact"
	AuthorTypeMember      AuthorType = "member"
	AuthorTypeAgent       AuthorType = "agent"
	AuthorTypeSystem      AuthorType = "system"
	AuthorTypeIntegration AuthorType = "integration"
)

/*
AuthorTypes returns every author type, in the order of the database
check.
*/
func AuthorTypes() []AuthorType {
	return []AuthorType{
		AuthorTypeContact, AuthorTypeMember, AuthorTypeAgent, AuthorTypeSystem, AuthorTypeIntegration,
	}
}

/*
Valid reports whether the author type is one of the author types.
*/
func (t AuthorType) Valid() bool {
	switch t {
	case AuthorTypeContact, AuthorTypeMember, AuthorTypeAgent, AuthorTypeSystem, AuthorTypeIntegration:
		return true
	}

	return false
}
