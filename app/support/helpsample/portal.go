/*
|--------------------------------------------------------------------------
| Sample Portal
|--------------------------------------------------------------------------
|
| The parts of the sample portal beyond articles and the changelog: the
| feature requests visitors vote on, grouped by where they stand, and the
| conversations a signed-in visitor had with the team. Both are written
| out as JSON beside the articles. Votes and replies are not kept; the
| pages show what the portal will look like.
|
*/

package helpsample

import (
	"encoding/json"
	"fmt"

	"github.com/dimmihq/dimmi/resources/views/helpcenter"
)

/*
status is a sample roadmap status with its feature requests.
*/
type status struct {
	Name  string `json:"name"`
	Slug  string `json:"slug"`
	Color string `json:"color"`
	Ideas []struct {
		Slug     string `json:"slug"`
		Title    string `json:"title"`
		Details  string `json:"details"`
		Icon     string `json:"icon"`
		Votes    int    `json:"votes"`
		Voted    bool   `json:"voted"`
		Comments int    `json:"comments"`
	} `json:"ideas"`
}

/*
conversation is a sample conversation; a message's author is "me" for
the visitor or the key of a person on the team.
*/
type conversation struct {
	ID       string `json:"id"`
	Subject  string `json:"subject"`
	Status   string `json:"status"`
	Updated  string `json:"updated"`
	Unread   bool   `json:"unread"`
	Preview  string `json:"preview"`
	Messages []struct {
		Author string `json:"author"`
		At     string `json:"at"`
		Body   string `json:"body"`
	} `json:"messages"`
}

/*
Visitor is the person the sample signs in as.
*/
func Visitor() helpcenter.Person {
	return helpcenter.Person{Name: "Ada Lovelace", Initials: "AL"}
}

/*
Feedback returns the feature requests grouped by status, in the order
the roadmap shows them.
*/
func (l *Library) Feedback() []helpcenter.Status {
	out := make([]helpcenter.Status, 0, len(l.statuses))

	for _, s := range l.statuses {
		v := helpcenter.Status{Name: s.Name, Slug: s.Slug, Color: s.Color, Count: len(s.Ideas)}
		for _, idea := range s.Ideas {
			v.Ideas = append(v.Ideas, helpcenter.Idea{
				Slug: idea.Slug, Title: idea.Title, Details: idea.Details, Icon: idea.Icon, Votes: idea.Votes,
				Voted: idea.Voted, Comments: idea.Comments,
			})
		}

		out = append(out, v)
	}

	return out
}

/*
Conversations returns the signed-in visitor's conversations, newest
first.
*/
func (l *Library) Conversations() []helpcenter.Conversation {
	out := make([]helpcenter.Conversation, 0, len(l.conversations))

	for _, c := range l.conversations {
		v := helpcenter.Conversation{
			ID: c.ID, Subject: c.Subject, Status: c.Status, Updated: c.Updated, Unread: c.Unread, Preview: c.Preview,
		}

		for _, m := range c.Messages {
			author, ok := person(m.Author)
			if !ok {
				author = Visitor()
			}

			v.Messages = append(v.Messages, helpcenter.Message{Author: author, Mine: !ok, Body: m.Body, At: m.At})
		}

		out = append(out, v)
	}

	return out
}

/*
Conversation returns the signed-in visitor's conversation with the id.
*/
func (l *Library) Conversation(id string) (helpcenter.Conversation, bool) {
	for _, c := range l.Conversations() {
		if c.ID == id {
			return c, true
		}
	}

	return helpcenter.Conversation{}, false
}

/*
readJSON decodes a JSON file of the sample content.
*/
func readJSON(name string, into any) error {
	data, err := content.ReadFile("content/" + name)
	if err != nil {
		return fmt.Errorf("read sample %s: %w", name, err)
	}

	if err := json.Unmarshal(data, into); err != nil {
		return fmt.Errorf("decode sample %s: %w", name, err)
	}

	return nil
}
