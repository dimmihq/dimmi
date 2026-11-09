/*
|--------------------------------------------------------------------------
| Knowledge Tools
|--------------------------------------------------------------------------
|
| What the team has written down: the knowledge the AI agent answers
| from, the help centers' articles with their drafts, and the procedures
| the AI agent follows. Operator reads drafts as well as what is live,
| since its job includes finding what is missing or wrong.
|
*/

package operator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/helpcenters"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
knowledgeTools are the tools that read knowledge, articles and
procedures.
*/
func (e *Engine) knowledgeTools() []tool {
	idSchema := func(field, about string) json.RawMessage {
		return schema(fmt.Sprintf(`{"type":"object","required":[%q],"properties":{%q:{"type":"string",`+
			`"description":%q}}}`, field, field, about))
	}
	return []tool{
		{spec: contracts.ToolSpec{
			Name: "search_knowledge", Description: "Search the knowledge the AI agent answers customers from: " +
				"published help articles and connected sources. Returns passages with titles and links.",
			Schema: schema(`{"type":"object","required":["query"],"properties":{"query":{"type":"string"}}}`),
		}, handle: e.searchKnowledge},
		{spec: contracts.ToolSpec{
			Name: "list_articles", Description: "List help center articles, drafts included, optionally " +
				"matching words in the title. Returns ids, status and whether unpublished changes wait.",
			Schema: schema(`{"type":"object","properties":{"query":{"type":"string"},` +
				`"status":{"type":"string","enum":["draft","published","archived"]}}}`),
		}, handle: e.listArticles},
		{spec: contracts.ToolSpec{
			Name: "get_article", Description: "Read one article's working copy in Markdown.",
			Schema: idSchema("article", "Article id, such as art_..."),
		}, handle: e.getArticle},
		{spec: contracts.ToolSpec{
			Name: "list_procedures", Description: "List the procedures the AI agent follows, with how often " +
				"each was used.",
			Schema: schema(`{"type":"object","properties":{}}`),
		}, handle: e.listProcedures},
		{spec: contracts.ToolSpec{
			Name: "get_procedure", Description: "Read one procedure's working copy and live copy as JSON.",
			Schema: idSchema("procedure", "Procedure id, such as prc_..."),
		}, handle: e.getProcedure},
	}
}

/*
searchKnowledge searches the agent's knowledge.
*/
func (e *Engine) searchKnowledge(ctx context.Context, s *session, args json.RawMessage) (result, error) {
	var in struct {
		Query string `json:"query"`
	}
	if err := decode(args, &in); err != nil {
		return result{}, err
	}
	if e.Services.Knowledge == nil {
		return result{}, fmt.Errorf("%w: knowledge is not available", errBadArguments)
	}
	found, err := e.Services.Knowledge.Search(ctx, s.m.Workspace.ID, in.Query, contracts.KnowledgeSearchOptions{
		Limit: 8, Visibility: []contracts.KnowledgeVisibility{
			contracts.KnowledgeVisibilityPublic, contracts.KnowledgeVisibilityAgentOnly,
		},
	})
	if err != nil {
		return result{}, err
	}
	var b strings.Builder
	for i, c := range found.Chunks {
		fmt.Fprintf(&b, "[%d] %s", i+1, c.Title)
		if c.ArticleID != nil {
			fmt.Fprintf(&b, " (article %s)", models.PublicID(models.PrefixArticle, *c.ArticleID))
		}
		fmt.Fprintf(&b, " %s\n%s\n\n", c.URL, truncate(c.Text, 1500))
	}
	if b.Len() == 0 {
		b.WriteString("Nothing in the knowledge matches.")
	}
	summary := fmt.Sprintf("Searched knowledge for %q: %d passages", in.Query, len(found.Chunks))
	return result{text: b.String(), step: Step{Tool: "search_knowledge", Summary: summary}}, nil
}

/*
listArticles lists the articles of every help center.
*/
func (e *Engine) listArticles(ctx context.Context, s *session, args json.RawMessage) (result, error) {
	var in struct {
		Query  string `json:"query"`
		Status string `json:"status"`
	}
	if err := decode(args, &in); err != nil {
		return result{}, err
	}
	centers, err := e.helpCenters(ctx, s)
	if err != nil {
		return result{}, err
	}
	var b strings.Builder
	count := 0
	for _, hc := range centers {
		list, err := e.Services.Articles.List(ctx, s.m, helpcenters.ArticleFilter{
			HelpCenterID: hc.ID, Status: in.Status, Query: in.Query, Limit: 100,
		})
		if err != nil {
			return result{}, err
		}
		fmt.Fprintf(&b, "Help center %s (id %s):\n", hc.Name, models.PublicID(models.PrefixHelpCenter, hc.ID))
		for _, a := range list {
			fmt.Fprintf(&b, "- %s · %s%s · updated %s (id %s)\n", a.Title, a.Status, pendingMark(a),
				a.UpdatedAt.UTC().Format("2006-01-02"), models.PublicID(models.PrefixArticle, a.ID))
		}
		count += len(list)
	}
	summary := fmt.Sprintf("Listed %d articles", count)
	return result{text: b.String(), step: Step{Tool: "list_articles", Summary: summary}}, nil
}

/*
pendingMark notes an article whose working copy is ahead of what is
live.
*/
func pendingMark(a models.Article) string {
	if a.HasUnpublishedChanges && a.Status == "published" {
		return " (unpublished changes)"
	}
	return ""
}

/*
helpCenters lists the workspace's help centers for a teammate who may
read articles.
*/
func (e *Engine) helpCenters(ctx context.Context, s *session) ([]models.HelpCenter, error) {
	if !s.m.Can("articles:read") {
		return nil, exceptions.ErrForbidden
	}
	var out []models.HelpCenter
	err := e.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		out, err = models.New(tx).ListHelpCenters(ctx, s.m.Workspace.ID)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("list help centers: %w", err)
	}
	return out, nil
}

/*
getArticle reads one article's working copy.
*/
func (e *Engine) getArticle(ctx context.Context, s *session, args json.RawMessage) (result, error) {
	id, err := e.idArg(args, "article", models.PrefixArticle)
	if err != nil {
		return result{}, err
	}
	a, err := e.Services.Articles.Show(ctx, s.m, id)
	if err != nil {
		return result{}, err
	}
	text := fmt.Sprintf("Article %s (id %s)\nStatus: %s%s\nVisibility: %s\nDescription: %s\n\n%s",
		a.Title, models.PublicID(models.PrefixArticle, a.ID), a.Status, pendingMark(a), a.Visibility,
		deref(a.Description), truncate(a.BodyMarkdown, 20000))
	return result{text: text, step: Step{Tool: "get_article", Summary: "Read the article " + a.Title}}, nil
}

/*
listProcedures lists the agent's procedures.
*/
func (e *Engine) listProcedures(ctx context.Context, s *session, _ json.RawMessage) (result, error) {
	list, err := e.Services.Procedures.List(ctx, s.m)
	if err != nil {
		return result{}, err
	}
	var b strings.Builder
	for _, p := range list {
		fmt.Fprintf(&b, "- %s · %s · used %d times (id %s)\n", p.Name, p.Status, p.Uses,
			models.PublicID(models.PrefixProcedure, p.ID))
	}
	if b.Len() == 0 {
		b.WriteString("No procedures yet.")
	}
	summary := fmt.Sprintf("Listed %d procedures", len(list))
	return result{text: b.String(), step: Step{Tool: "list_procedures", Summary: summary}}, nil
}

/*
getProcedure reads one procedure.
*/
func (e *Engine) getProcedure(ctx context.Context, s *session, args json.RawMessage) (result, error) {
	id, err := e.idArg(args, "procedure", models.PrefixProcedure)
	if err != nil {
		return result{}, err
	}
	p, err := e.Services.Procedures.Show(ctx, s.m, id)
	if err != nil {
		return result{}, err
	}
	text := fmt.Sprintf("Procedure %s (id %s), %s\nWorking copy: %s\nLive copy: %s", p.Name,
		models.PublicID(models.PrefixProcedure, p.ID), p.Status, truncate(string(p.Draft), 20000),
		truncate(string(p.Live), 20000))
	return result{text: text, step: Step{Tool: "get_procedure", Summary: "Read the procedure " + p.Name}}, nil
}

/*
idArg reads one public id argument of the prefix.
*/
func (e *Engine) idArg(args json.RawMessage, field string, prefix models.Prefix) (ids.UUID, error) {
	var in map[string]string
	if err := decode(args, &in); err != nil {
		return ids.UUID{}, err
	}
	return e.publicID(in[field], prefix)
}

/*
publicID reads a public id of the prefix; anything else is not found.
*/
func (e *Engine) publicID(public string, prefix models.Prefix) (ids.UUID, error) {
	id, err := e.IDs.Decode(string(prefix), strings.TrimSpace(public))
	if err != nil {
		return ids.UUID{}, exceptions.ErrNotFound
	}
	return id, nil
}
