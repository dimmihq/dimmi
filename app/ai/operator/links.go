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
| What customers can open has two: its page in dimmi, for the team, and
| its public page on the help center, which exists once it is published.
|
*/

package operator

import (
	"context"
	"fmt"
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
	linkChangelog    = "changelog"
	linkFeedback     = "feedback"
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

/*
publicRoot is where a help center's public pages start, empty when the
help center cannot be found.
*/
func (e *Engine) publicRoot(ctx context.Context, s *session, helpCenter ids.UUID) string {
	centers, err := e.helpCenters(ctx, s)
	if err != nil {
		return ""
	}
	for _, c := range centers {
		if c.ID == helpCenter {
			return e.Addresses.Root(c)
		}
	}
	return ""
}

/*
links writes a record's two links: its page in dimmi, and its public
page, or a line saying it has none yet when public is empty.
*/
func links(app, public string) string {
	if public == "" {
		return fmt.Sprintf("Link in dimmi, for the team: %s\nPublic link: none yet, it is not published.\n", app)
	}
	return fmt.Sprintf("Link in dimmi, for the team: %s\nPublic link, what customers see: %s\n", app, public)
}

/*
articleLinks are an article's links: its editor, and its page on the
help center once published.
*/
func (e *Engine) articleLinks(ctx context.Context, s *session, a models.Article) string {
	public := ""
	if root := e.publicRoot(ctx, s, a.HelpCenterID); root != "" && a.Status == "published" {
		public = root + "/articles/" + a.Slug
	}
	return links(e.link(s, linkArticle, models.PrefixArticle, a.ID), public)
}
