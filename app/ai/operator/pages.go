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
	"github.com/dimmihq/dimmi/app/actions/series"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/prompts"
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
	PageWorkflow       = "workflow"
	PageSeries         = "series"
)

/*
pageKinds is every kind of page Operator is told about.
*/
func (e *Engine) pageKinds() map[string]pageKind {
	return map[string]pageKind{
		PageArticle:      {prefix: models.PrefixArticle, describe: e.articlePage},
		PageProcedure:    {prefix: models.PrefixProcedure, describe: e.procedurePage},
		PageConversation: {prefix: models.PrefixConversation, describe: e.conversationPage},
		PageWorkflow:     {prefix: models.PrefixWorkflow, describe: e.workflowPage},
		PageSeries:       {prefix: models.PrefixSeries, describe: e.seriesPage},
		PageHelpCenter: {
			prefix: models.PrefixHelpCenter,
			describe: e.apiPage(apiSource{
				what: "help center", tool: "dimmi_get_help_center_by_id", argument: "help_center",
				prefix: models.PrefixHelpCenter, public: func(r apiRecord, _ string) string { return r.URL },
				live: func(r apiRecord) bool { return r.Status == "active" },
			}),
		},
		PageFeatureRequest: {
			prefix: models.PrefixFeedbackPost,
			describe: e.apiPage(apiSource{
				what: "feature request", tool: "dimmi_get_feedback_post_by_id", argument: "post",
				prefix: models.PrefixFeedbackPost, page: linkFeedback,
				public: func(r apiRecord, root string) string {
					return fmt.Sprintf("%s/feedback/%d-%s", root, r.Number, r.Slug)
				},
				live: func(r apiRecord) bool { return r.Moderation == "approved" },
			}),
		},
		PageChangelogEntry: {
			prefix: models.PrefixChangelogEntry,
			describe: e.apiPage(apiSource{
				what: "changelog entry", tool: "dimmi_get_changelog_entry_by_id", argument: "entry",
				prefix: models.PrefixChangelogEntry, page: linkChangelog,
				public: func(r apiRecord, root string) string { return root + "/changelog/" + r.Slug },
				live:   func(r apiRecord) bool { return r.Status == "published" },
			}),
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
	fmt.Fprintf(&b, "The help article %q is open.\narticle_id: %s\nStatus: %s%s\n%s", a.Title, public,
		a.Status, pendingMark(a), e.articleLinks(ctx, s, a))
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
	return fmt.Sprintf("The procedure %q is open.\nprocedure_id: %s\nStatus: %s\nLink: %s\nWorking copy: %s\n"+
		"To change it, call propose_changes with a procedure.edit_draft for procedure_id %s.", p.Name, public,
		p.Status, e.link(s, linkProcedure, models.PrefixProcedure, p.ID), truncate(string(p.Draft), 20000),
		public), nil
}

/*
workflowPage describes an open workflow in its builder: everything
get_workflow_by_id reads, with the suggestion open on it when there is
one.
*/
func (e *Engine) workflowPage(ctx context.Context, s *session, id ids.UUID) (string, error) {
	if e.Services.Workflows == nil {
		return "", nil
	}
	wf, err := e.Services.Workflows.Show(ctx, s.m, id)
	if err != nil {
		return "", err
	}
	text, err := e.workflowText(ctx, s, wf)
	if err != nil {
		return "", err
	}
	return "The workflow builder is open on this workflow.\n" + text, nil
}

/*
seriesPage describes an open series in its builder.
*/
func (e *Engine) seriesPage(ctx context.Context, s *session, id ids.UUID) (string, error) {
	if e.Services.Series == nil {
		return "", nil
	}
	row, err := e.Services.Series.Show(ctx, s.m, id)
	if err != nil {
		return "", err
	}
	text, err := e.seriesText(s, row)
	if err != nil {
		return "", err
	}
	return "The series builder is open on this series.\n" + e.selectedStep(s, row) + text, nil
}

/*
selectedStep says which step the teammate has selected in the series
builder, as the working copy shows it, so "this step" means that one.
*/
func (e *Engine) selectedStep(s *session, row models.Series) string {
	id := strings.TrimSpace(s.viewing.Step)
	if id == "" {
		return ""
	}
	spec, err := series.SpecOf(row)
	if err != nil {
		return ""
	}
	if suggested, ok := s.suggestedSeries(row.ID); ok {
		spec = suggested
	}
	for _, n := range spec.Nodes {
		if n.ID != id {
			continue
		}
		line := fmt.Sprintf("Selected step: %s · %s", n.ID, n.Kind)
		if len(n.Settings) > 0 {
			line += " · settings " + truncate(compactJSON(n.Settings), 2000)
		}
		return line + "\nWhen the teammate says this step, they mean step " + n.ID + ".\n"
	}
	return ""
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
	fmt.Fprintf(&b, "Link: %s\n", e.conversationLink(s, id))
	writeTimeline(&b, timeline)
	if hits, err := e.alikeTo(ctx, s, id, upFront); err == nil && len(hits) > 0 {
		b.WriteString("\nThe conversations most like this one, and what the team did with them. Base any " +
			"teammate, team, tag or priority you recommend on these, and cite them by number:\n")
		b.WriteString(e.relatedText(s, hits) + "\n")
	}
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
	b.WriteString(prompts.Task(taskReplyDrafting, nil))
	return b.String()
}

/*
apiSource says how a page with no reader of Operator's own is read: by
which API read, under which argument, which id it carries, the page it
has in dimmi, and its public address from the record and the root of
its help center.
*/
type apiSource struct {
	what, tool, argument string
	prefix               models.Prefix
	page                 string
	public               func(record apiRecord, root string) string
	live                 func(record apiRecord) bool
}

/*
apiRecord is what a link needs from a record the API returned.
*/
type apiRecord struct {
	Slug         string `json:"slug"`
	Status       string `json:"status"`
	URL          string `json:"url"`
	Number       int64  `json:"number"`
	Moderation   string `json:"moderation"`
	HelpCenterID string `json:"help_center_id"`
}

/*
apiPage describes a page Operator has no reader of its own for, by
reading the record from the API route the page calls, as the teammate,
with its links.
*/
func (e *Engine) apiPage(src apiSource) func(ctx context.Context, s *session, id ids.UUID) (string, error) {
	return func(ctx context.Context, s *session, id ids.UUID) (string, error) {
		if e.Catalog == nil {
			return "", nil
		}
		public := models.PublicID(src.prefix, id)
		args, _ := json.Marshal(map[string]string{src.argument: public})
		text, err := e.Catalog.Call(ctx, s.m, CatalogCall{Name: src.tool, Args: args})
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("A %s is open.\n%s_id: %s\n%s%s", src.what, strings.ReplaceAll(src.what, " ", "_"),
			public, e.apiLinks(ctx, s, src, id), truncate(text, 20000)), nil
	}
}

/*
apiLinks are the links of a page read from the API: its page in dimmi
when it has one, and its public page once it is live.
*/
func (e *Engine) apiLinks(ctx context.Context, s *session, src apiSource, id ids.UUID) string {
	args, _ := json.Marshal(map[string]string{src.argument: models.PublicID(src.prefix, id)})
	raw, err := e.Catalog.Call(ctx, s.m, CatalogCall{Name: src.tool, Args: args, Raw: true})
	if err != nil {
		return ""
	}
	var body struct {
		Data apiRecord `json:"data"`
	}
	if json.Unmarshal([]byte(raw), &body) != nil {
		return ""
	}
	r := body.Data
	app := strings.TrimRight(e.AppURL, "/") + "/d/" + s.m.Workspace.Slug + "/" + linkArticle
	if src.page != "" {
		app = e.link(s, src.page, src.prefix, id)
	}
	root := ""
	if center, err := e.IDs.Decode(string(models.PrefixHelpCenter), r.HelpCenterID); err == nil {
		root = e.publicRoot(ctx, s, center)
	}
	if !src.live(r) || (root == "" && r.URL == "") {
		return links(app, "")
	}
	return links(app, src.public(r, root))
}
