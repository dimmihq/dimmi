/*
|--------------------------------------------------------------------------
| Ephemeral Events
|--------------------------------------------------------------------------
|
| Some events only matter to people watching right now, such as someone
| typing. They are never stored, never audited and never sent to
| webhooks, so only after-commit listeners, which tell open screens,
| ever hear them.
|
*/

package events

/*
Ephemeral marks an event that is never stored. Embed it in the event's
struct.
*/
type Ephemeral struct{}

func (Ephemeral) ephemeral() {}

/*
IsEphemeral reports whether the event embeds Ephemeral, for listeners
that must not keep it, such as a broadcaster's history.
*/
func IsEphemeral(e Event) bool {
	_, ok := e.(interface{ ephemeral() })

	return ok
}
