/*
|--------------------------------------------------------------------------
| Participant Role
|--------------------------------------------------------------------------
|
| Beside its contact, a conversation can have people copied on it, as on
| an email, and people who took part, as in a chat channel.
|
*/

package enums

/*
ParticipantRole is how an extra contact is on a conversation.
*/
type ParticipantRole string

/*
The participant roles.
*/
const (
	ParticipantRoleCC          ParticipantRole = "cc"
	ParticipantRoleParticipant ParticipantRole = "participant"
)

/*
ParticipantRoles returns every role, in the order of the database check.
*/
func ParticipantRoles() []ParticipantRole {
	return []ParticipantRole{ParticipantRoleCC, ParticipantRoleParticipant}
}

/*
Valid reports whether the role is one of the roles.
*/
func (r ParticipantRole) Valid() bool {
	switch r {
	case ParticipantRoleCC, ParticipantRoleParticipant:
		return true
	}

	return false
}
