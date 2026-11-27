/*
|--------------------------------------------------------------------------
| Sample Help Center Pages
|--------------------------------------------------------------------------
|
| The sample help center's demo of a reader's conversations with the
| team, which only the sample shows.
|
*/

package helpcenter

import "github.com/dimmihq/dimmi/framework/view"

/*
ResultVars is a search hit.
*/
type ResultVars struct {
	// Article or changelog.
	Kind string
	// Its title.
	Title string
	// Its address.
	URL string
	// The text that matched.
	Excerpt string
	// Where it lives, such as its collection.
	Trail string
}

/*
ConversationVars is one of the sample reader's conversations.
*/
type ConversationVars struct {
	// Its id.
	ID string
	// Its subject.
	Subject string
	// Its last message, shortened.
	Preview string
	// Where it stands.
	Status string
	// When it last changed.
	Updated string
	// It has a reply the reader has not read.
	Unread bool
	// Its page.
	URL string
	// Where the reply form posts.
	ReplyURL string
	// Its thread.
	Messages []MessageVars
}

func (c Center) conversations(list []Conversation) []ConversationVars {
	out := make([]ConversationVars, 0, len(list))
	for _, conv := range list {
		out = append(out, c.conversation(conv))
	}
	return out
}

func (c Center) conversation(conv Conversation) ConversationVars {
	return ConversationVars{
		ID: conv.ID, Subject: conv.Subject, Preview: conv.Preview, Status: conv.Status, Updated: conv.Updated,
		Unread: conv.Unread, URL: c.url("/conversations/" + conv.ID),
		ReplyURL: c.url("/conversations/" + conv.ID + "/messages"),
		Messages: messages(conv.Messages),
	}
}

/*
SignInPage is the sample help center's sign-in.
*/
func SignInPage(c Center, all []Collection, _ bool) view.Component {
	m := Meta{
		Title: "Sign in | " + c.Product, Section: "conversations",
		Trail: []Crumb{{Title: "Sign in", Path: "/sign-in"}}, NoIndex: true,
	}
	return Render(c, "demo_sign_in", m, map[string]any{"collections": c.collections(all)})
}

/*
ConversationsPage is the sample reader's inbox.
*/
func ConversationsPage(c Center, conversations []Conversation) view.Component {
	m := Meta{
		Title: "Your conversations | " + c.Product, Section: "conversations",
		Trail: []Crumb{{Title: "Conversations", Path: "/conversations"}}, NoIndex: true,
	}
	return Render(c, "demo_conversations", m, map[string]any{"conversations": c.conversations(conversations)})
}

/*
ConversationPage shows one of the sample reader's conversations.
*/
func ConversationPage(c Center, all []Conversation, conv Conversation) view.Component {
	m := Meta{
		Title: conv.Subject + " | " + c.Product, Section: "conversations",
		Trail: []Crumb{
			{Title: "Conversations", Path: "/conversations"},
			{Title: conv.Subject, Path: "/conversations/" + conv.ID},
		}, NoIndex: true,
	}
	return Render(
		c, "demo_conversation", m,
		map[string]any{"conversations": c.conversations(all), "conversation": c.conversation(conv)},
	)
}

/*
NewConversationPage starts a sample conversation.
*/
func NewConversationPage(c Center, all []Conversation) view.Component {
	m := Meta{
		Title: "New conversation | " + c.Product, Section: "conversations",
		Trail: []Crumb{
			{Title: "Conversations", Path: "/conversations"},
			{Title: "New conversation", Path: "/conversations/new"},
		}, NoIndex: true,
	}
	return Render(c, "demo_conversation_new", m, map[string]any{"conversations": c.conversations(all)})
}
