/*
|--------------------------------------------------------------------------
| Pages
|--------------------------------------------------------------------------
|
| Operator sits beside many pages, and on each it is told what the
| teammate has open, in full, so it never looks the page up or copies
| an id from memory. This file is the one place that says, for each kind
| of page, which id it carries and what Operator is told about it.
|
| The app sends the page's kind, the id it shows and the suggestion on
| it. A kind's describe reads the record as the teammate, with their
| permissions, from the same place the page itself reads it: Operator's
| own readers where it has one that shapes the text for a model, else
| the API route the page calls. To give Operator a new page, add one
| entry here and have the page say what it has open; nothing else
| changes.
|
*/

package operator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/actions/messages"
	"github.com/dimmihq/dimmi/app/models"
)

/*
pageKind is one kind of page Operator is told about: the id it carries
and how what it shows is described.
*/
type pageKind struct {
	prefix   models.Prefix
	describe func(ctx context.Context, s *session, id ids.UUID) (string, error)
}

/*
The kinds of page the app may say a teammate has open.
*/
const (
	PageArticle        = "article"
	PageProcedure      = "procedure"
	PageConversation   = "conversation"
	PageHelpCenter     = "help_center"
	PageFeatureRequest = "feature_request"
	PageChangelogEntry = "changelog_entry"
)

/*
pageKinds is every kind of page Operator is told about.
*/
func (e *Engine) pageKinds() map[string]pageKind {
	return map[string]pageKind{
		PageArticle:      {prefix: models.PrefixArticle, describe: e.articlePage},
		PageProcedure:    {prefix: models.PrefixProcedure, describe: e.procedurePage},
		PageConversation: {prefix: models.PrefixConversation, describe: e.conversationPage},
		PageHelpCenter: {
			prefix:   models.PrefixHelpCenter,
			describe: e.apiPage("help center", "dimmi_help_centers_get", "help_center", models.PrefixHelpCenter),
		},
		PageFeatureRequest: {
			prefix: models.PrefixFeedbackPost,
			describe: e.apiPage("feature request", "dimmi_feedback_posts_get", "post",
				models.PrefixFeedbackPost),
		},
		PageChangelogEntry: {
			prefix: models.PrefixChangelogEntry,
			describe: e.apiPage("changelog entry", "dimmi_changelog_entries_get", "entry",
				models.PrefixChangelogEntry),
		},
	}
}

/*
articlePage describes an open article: its id, status and text, or how
to change it when a suggestion on it carries the text instead.
*/
func (e *Engine) articlePage(ctx context.Context, s *session, id ids.UUID) (string, error) {
	a, err := e.Services.Articles.Show(ctx, s.m, id)
	if err != nil {
		return "", err
	}
	public := models.PublicID(models.PrefixArticle, a.ID)
	var b strings.Builder
	fmt.Fprintf(&b, "The help article %q is open.\narticle_id: %s\nStatus: %s%s\n", a.Title, public, a.Status,
		pendingMark(a))
	if _, suggested := s.suggested(a.ID); !suggested {
		fmt.Fprintf(&b, "The article as it reads now:\n%s\n", truncate(a.BodyMarkdown, 20000))
	}
	fmt.Fprintf(&b, "To change it, call propose_article_edits with article_id %s.", public)
	return b.String(), nil
}

/*
procedurePage describes an open procedure: its id, status and working
copy.
*/
func (e *Engine) procedurePage(ctx context.Context, s *session, id ids.UUID) (string, error) {
	p, err := e.Services.Procedures.Show(ctx, s.m, id)
	if err != nil {
		return "", err
	}
	public := models.PublicID(models.PrefixProcedure, p.ID)
	return fmt.Sprintf("The procedure %q is open.\nprocedure_id: %s\nStatus: %s\nWorking copy: %s\n"+
		"To change it, call propose_changes with a procedure.edit_draft for procedure_id %s.", p.Name, public,
		p.Status, truncate(string(p.Draft), 20000), public), nil
}

/*
conversationPage describes an open conversation: its details and latest
messages. What a customer wrote is now in the run, so nothing the run
proposes applies on its own.
*/
func (e *Engine) conversationPage(ctx context.Context, s *session, id ids.UUID) (string, error) {
	loaded, err := e.Services.Show.Handle(ctx, conversations.ShowConversationInput{
		Membership: s.m, Ref: conversations.Ref{ID: id},
	})
	if err != nil {
		return "", err
	}
	timeline, err := e.Services.Messages.Handle(ctx, messages.ListMessagesInput{
		Membership: s.m, Ref: conversations.Ref{ID: id}, Limit: timelineSize,
	})
	if err != nil {
		return "", err
	}
	s.tainted = true
	var b strings.Builder
	fmt.Fprintf(&b, "Conversation #%d is open.\nconversation_number: %d\n", loaded.Conversation.Number,
		loaded.Conversation.Number)
	writeConversation(&b, loaded)
	writeTimeline(&b, timeline)
	return b.String(), nil
}

/*
apiPage describes a page Operator has no reader of its own for, by
reading the record from the API route the page calls, as the teammate.
*/
func (e *Engine) apiPage(what, tool, argument string, prefix models.Prefix,
) func(ctx context.Context, s *session, id ids.UUID) (string, error) {
	return func(ctx context.Context, s *session, id ids.UUID) (string, error) {
		if e.Catalog == nil {
			return "", nil
		}
		public := models.PublicID(prefix, id)
		args, _ := json.Marshal(map[string]string{argument: public})
		text, err := e.Catalog.Call(ctx, s.m, CatalogCall{Name: tool, Args: args})
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("A %s is open.\n%s_id: %s\n%s", what, strings.ReplaceAll(what, " ", "_"), public,
			truncate(text, 20000)), nil
	}
}
