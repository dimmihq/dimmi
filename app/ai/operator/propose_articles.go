/*
|--------------------------------------------------------------------------
| Article Suggestions
|--------------------------------------------------------------------------
|
| Suggesting a fix to a help article, or a new one, each through a tool
| of its own. A tool that does one thing can require what that thing
| needs, so a model cannot leave out the article or its edits the way
| it can in a tool that takes every kind of change. Both end in the
| same changeset a teammate reviews.
|
*/

package operator

import (
	"context"
	"encoding/json"

	"github.com/dimmihq/dimmi/app/contracts"
)

/*
The names of the tools that suggest changes to help articles.
*/
const (
	toolArticleEdit = "propose_article_edit"
	toolArticleNew  = "propose_new_article"
)

/*
articleEditSchema is the schema of a suggested fix to one article.
*/
const articleEditSchema = `{"type":"object","required":["article_id","edits","suggestion_title","reason"],"properties":{
"article_id":{"type":"string","description":"The id of the article to fix, as get_article_by_id and list_articles ` +
	`show it (art_...). Never its title."},
"edits":{"type":"array","minItems":1,"maxItems":20,"description":"The passages to replace. Read the article ` +
	`with get_article_by_id first.","items":{"type":"object","required":["find","replace"],"properties":{
"find":{"type":"string","description":"The passage as the article has it now, copied word for word from ` +
	`get_article_by_id: one whole sentence or list item is best. It is replaced everywhere it appears."},
"replace":{"type":"string","description":"What the passage should say instead, in Markdown. Empty to ` +
	`remove the passage."}}}},
"suggestion_title":{"type":"string","maxLength":200,"description":"What the fix does in a few everyday words, like ` +
	`'Fix the refund window in the returns guide'. Not the article's title."},
"reason":{"type":"string","maxLength":2000,"description":"` + reasonHelp + `"},
"dedup_key":{"type":"string","maxLength":200,"description":"A stable key for this fix, like ` +
	`'stale:art_...'. A suggestion with the same key that still waits, or was declined in the last 30 days, ` +
	`is not made again."},
"new_article_title":{"type":"string","maxLength":150,"description":"Only to rename the article: its new ` +
	`title."}}}`

/*
articleNewSchema is the schema of a suggested new article.
*/
const articleNewSchema = `{"type":"object",
"required":["article_title","article_body","suggestion_title","reason"],"properties":{
"article_title":{"type":"string","maxLength":150,"description":"The new article's title, as readers see it."},
"article_body":{"type":"string","description":"The whole article in Markdown."},
"suggestion_title":{"type":"string","maxLength":200,"description":"What the suggestion is in a few everyday ` +
	`words, like 'Add a guide to SSO setup'."},
"reason":{"type":"string","maxLength":2000,"description":"` + reasonHelp + `"},
"visibility":{"type":"string","enum":["public","agent_only"],"description":"public for customers, or ` +
	`agent_only for a snippet only the AI agent reads. Public by default."},
"dedup_key":{"type":"string","maxLength":200,"description":"A stable key for this article, like ` +
	`'gap:sso-setup'. A suggestion with the same key that still waits, or was declined in the last 30 days, ` +
	`is not made again."},
"collection_id":{"type":"string","description":"Collection id (col_...); leave out for none."},
"help_center_id":{"type":"string","description":"Help center id (hc_...); leave out for the default."}}}`

/*
articleTools are the tools that suggest changes to help articles.
*/
func (e *Engine) articleTools() []tool {
	return []tool{
		{
			spec: contracts.ToolSpec{
				Name: toolArticleEdit,
				Description: "Suggest a fix to one help article: the passages to replace, each quoted from " +
					"get_article_by_id with what it should say instead. A teammate reviews it; the article's draft " +
					"changes only when they approve. Call it once per article.",
				Schema: schema(articleEditSchema),
			}, writes: []Kind{KindArticleEdit}, handle: e.proposeArticleEdit,
			was: map[string]string{"title": "suggestion_title", "new_title": "new_article_title"},
		},
		{
			spec: contracts.ToolSpec{
				Name: toolArticleNew,
				Description: "Suggest a new help article as a draft a teammate reviews. Call it once per " +
					"article.",
				Schema: schema(articleNewSchema),
			}, writes: []Kind{KindArticleNew}, handle: e.proposeArticle,
			was: map[string]string{"title": "suggestion_title", "body": "article_body"},
		},
	}
}

/*
proposeArticleEdit suggests a fix to one article.
*/
func (e *Engine) proposeArticleEdit(ctx context.Context, s *session, args json.RawMessage) (result, error) {
	var in struct {
		ArticleID string       `json:"article_id"`
		Article   string       `json:"article"`
		Edits     passageEdits `json:"edits"`
		Title     string       `json:"suggestion_title"`
		Reason    string       `json:"reason"`
		DedupKey  string       `json:"dedup_key"`
		NewTitle  string       `json:"new_article_title"`
		Body      string       `json:"body"`
	}
	if err := decode(args, &in); err != nil {
		return result{}, err
	}
	return e.propose(ctx, s, proposal{
		Title: in.Title, Reason: in.Reason, DedupKey: in.DedupKey,
		Operations: []opInput{{
			Kind: KindArticleEdit, Article: either(in.ArticleID, in.Article), Edits: in.Edits,
			Title: in.NewTitle, Body: in.Body,
		}},
	})
}

/*
proposeArticle suggests a new article.
*/
func (e *Engine) proposeArticle(ctx context.Context, s *session, args json.RawMessage) (result, error) {
	var in struct {
		ArticleTitle string `json:"article_title"`
		Body         string `json:"article_body"`
		Title        string `json:"suggestion_title"`
		Reason       string `json:"reason"`
		Visibility   string `json:"visibility"`
		DedupKey     string `json:"dedup_key"`
		Collection   string `json:"collection"`
		HelpCenter   string `json:"help_center"`
		CollectionID string `json:"collection_id"`
		HelpCenterID string `json:"help_center_id"`
	}
	if err := decode(args, &in); err != nil {
		return result{}, err
	}
	if in.Title == "" {
		in.Title = in.ArticleTitle
	}
	return e.propose(ctx, s, proposal{
		Title: in.Title, Reason: in.Reason, DedupKey: in.DedupKey,
		Operations: []opInput{{
			Kind: KindArticleNew, Title: in.ArticleTitle, Body: in.Body, Visibility: in.Visibility,
			Collection: either(in.CollectionID, in.Collection), HelpCenter: either(in.HelpCenterID, in.HelpCenter),
		}},
	})
}
