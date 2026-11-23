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
	b.WriteString("\n" + e.replyGuide(s))
	return b.String(), nil
}

/*
replyGuide is how this workspace replies to customers, for a draft
Operator writes on a conversation: the product, tone, languages and
guidance the workspace gave its AI agent, and how to write the draft.
A draft is the one thing Operator writes for a customer, so it follows
the workspace's voice and never Operator's own way of talking to
teammates.
*/
func (e *Engine) replyGuide(s *session) string {
	a := s.agent
	product := strings.TrimSpace(deref(a.ProductName))
	if product == "" {
		product = s.m.Workspace.Name
	}
	var b strings.Builder
	b.WriteString("How this workspace replies to customers. Follow it for every reply you draft with " +
		"draft_reply; it replaces your own way of writing, which is only for teammates.\n")
	fmt.Fprintf(&b, "Product: %s\nTone: %s\n", product, a.Tone)
	if len(a.Languages) > 0 {
		fmt.Fprintf(&b, "Languages the team replies in: %s. Default: %s.\n", strings.Join(a.Languages, ", "),
			a.DefaultLanguage)
	}
	if guidance := strings.TrimSpace(a.Guidance); guidance != "" {
		fmt.Fprintf(&b, "The workspace's own guidance, to follow exactly:\n%s\n", truncate(guidance, 8000))
	}
	b.WriteString(replyDrafting)
	return b.String()
}

/*
replyDrafting is how Operator drafts a reply to a customer.
*/
const replyDrafting = `How to draft a reply:
- Draft only when the teammate asks for a reply, or the conversation waits on the team.
- Answer the customer's latest message. Lead with the fix or the answer, then the reason.
- Reply in the language the customer writes in when the team replies in it, else the default.
- Check every fact about the product with search_knowledge first. Never state a price, limit,
  date, policy or step you did not find there or in this conversation. When you cannot confirm
  something, leave it out of the draft and tell the teammate what to check.
- Never promise what only a person can decide: a refund, a credit, an exception, a date.
- Write as the team, to the customer, in the workspace's tone. Never mention Operator, an AI,
  internal notes, tags, ids or what teammates said to each other.
- Match the channel: an email opens with a greeting and closes with a sign-off; a chat reply
  is a few short lines with neither.
- Keep it as short as the answer allows. No filler, no apology unless something went wrong.
- Call draft_reply once with the whole reply. Then tell the teammate in one line what you
  drafted, and in one more anything they should check before sending.`

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
