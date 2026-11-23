/*
|--------------------------------------------------------------------------
| Links
|--------------------------------------------------------------------------
|
| A teammate asks Operator for a link, and Operator cites conversations
| and articles in what it writes. It cannot build an address: it does not
| know the app's address or the workspace's, and in a chat it sees short
| stand-ins for ids. So every record it reads comes with its link,
| written here, and it is told to use that link and never make one up.
|
*/

package operator

import (
	"strings"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
)

/*
The pages a link opens, by the path under the workspace.
*/
const (
	linkConversation = "inbox/all"
	linkArticle      = "help-center"
	linkProcedure    = "procedures"
	linkRoutine      = "routines"
)

/*
link is the address of a record's page in the app, for the teammate's
workspace.
*/
func (e *Engine) link(s *session, page string, prefix models.Prefix, id ids.UUID) string {
	return strings.TrimRight(e.AppURL, "/") + "/d/" + s.m.Workspace.Slug + "/" + page + "/" +
		models.PublicID(prefix, id)
}

/*
conversationLink is the address of a conversation.
*/
func (e *Engine) conversationLink(s *session, id ids.UUID) string {
	return e.link(s, linkConversation, models.PrefixConversation, id)
}
