/*
|--------------------------------------------------------------------------
| Portal Community
|--------------------------------------------------------------------------
|
| What the portal shows beside its articles: feature requests grouped by
| where they stand, and the signed-in visitor's conversations with the
| team.
|
*/

package helpcenter

/*
Conversation is one of the signed-in visitor's conversations with the
team, as their inbox lists it and as its own page shows it.
*/
type Conversation struct {
	ID       string
	Subject  string
	Preview  string
	Status   string
	Updated  string
	Unread   bool
	Messages []Message
}

/*
Message is a message in a conversation; Mine says the visitor sent it.
*/
type Message struct {
	Author Person
	Mine   bool
	Body   string
	At     string
}
