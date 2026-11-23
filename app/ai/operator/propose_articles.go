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
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/app/contracts"
)

/*
The names of the tools that suggest changes to help articles.
*/
const (
	toolArticleEdit = "propose_article_edits"
	toolArticleNew  = "propose_new_article"
)

/*
articleEditsSchema is the schema of suggested fixes to articles: a list,
so every fix found goes out in one call.
*/
const articleEditsSchema = `{"type":"object","required":["fixes"],"properties":{"fixes":{"type":"array",
"minItems":1,"maxItems":20,"description":"Every fix you found, one entry per article. Send them all in this ` +
	`one call: each becomes its own suggestion, and one that is refused does not stop the others.",
"items":{"type":"object","required":["article_id","edits","suggestion_title","reason"],"properties":{
"article_id":{"type":"string","description":"The id of the article to fix, as get_article_by_id and list_articles ` +
	`show it (art_...). Never its title."},
"edits":{"type":"array","minItems":1,"maxItems":20,"description":"The passages of that article to replace.",
"items":{"type":"object","required":["find","replace"],"properties":{
"find":{"type":"string","description":"The passage as that article has it now, copied word for word from ` +
	`get_article_by_id: one whole sentence or list item is best. It is replaced everywhere it appears."},
"replace":{"type":"string","description":"What the passage should say instead, in Markdown. Empty to ` +
	`remove the passage. Must differ from find."}}}},
"suggestion_title":{"type":"string","maxLength":200,"description":"What the fix does in a few everyday words, like ` +
	`'Fix the refund window in the returns guide'. Not the article's title."},
"reason":{"type":"string","maxLength":2000,"description":"` + reasonHelp + `"},
"dedup_key":{"type":"string","maxLength":200,"description":"A stable key for this fix, like ` +
	`'stale:art_...'. A suggestion with the same key that still waits, or was declined in the last 30 days, ` +
	`is not made again."},
"new_article_title":{"type":"string","maxLength":150,"description":"Only to rename the article: its new ` +
	`title."}}}}}}`

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
				Description: "Suggest fixes to help articles, all in one call: for each article, the passages to " +
					"replace, each quoted from get_article_by_id with what it should say instead. Each fix " +
					"becomes its own suggestion a teammate reviews; an article's draft changes only when they " +
					"approve. Call it once, with every fix you found.",
				Schema: schema(articleEditsSchema),
			}, writes: []Kind{KindArticleEdit}, handle: e.proposeArticleEdits,
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
articleFix is one suggested fix to one article, as the model sends it.
*/
type articleFix struct {
	ArticleID string       `json:"article_id"`
	Article   string       `json:"article"`
	Edits     passageEdits `json:"edits"`
	Title     string       `json:"suggestion_title"`
	Reason    string       `json:"reason"`
	DedupKey  string       `json:"dedup_key"`
	NewTitle  string       `json:"new_article_title"`
	Body      string       `json:"body"`
}

/*
proposal is the fix as a proposal of one operation.
*/
func (f articleFix) proposal() proposal {
	return proposal{
		Title: f.Title, Reason: f.Reason, DedupKey: f.DedupKey,
		Operations: []opInput{{
			Kind: KindArticleEdit, Article: either(f.ArticleID, f.Article), Edits: f.Edits,
			Title: f.NewTitle, Body: f.Body,
		}},
	}
}

/*
articleFixes are the fixes of one call, read whether the model sent a
list, one fix alone, or either encoded as a string.
*/
type articleFixes []articleFix

/*
UnmarshalJSON reads the fixes in any of the shapes models send.
*/
func (f *articleFixes) UnmarshalJSON(raw []byte) error {
	raw = objectArg(raw)
	var list []articleFix
	if err := json.Unmarshal(raw, &list); err == nil {
		*f = list
		return nil
	}
	var one articleFix
	if err := json.Unmarshal(raw, &one); err != nil {
		return fmt.Errorf("%w: \"fixes\" is a list of fixes, each with article_id, edits, suggestion_title "+
			"and reason", errBadArguments)
	}
	*f = articleFixes{one}
	return nil
}

/*
proposeArticleEdits suggests fixes to articles, each as a suggestion of
its own. A call with one fix answers as that fix does; with several,
every fix is tried, the ones that hold are suggested, and the model is
told which were not and why, so it sends only those again.
*/
func (e *Engine) proposeArticleEdits(ctx context.Context, s *session, args json.RawMessage) (result, error) {
	var in struct {
		Fixes articleFixes `json:"fixes"`
		articleFix
	}
	if err := decode(args, &in); err != nil {
		return result{}, err
	}
	fixes := in.Fixes
	if len(fixes) == 0 {
		fixes = articleFixes{in.articleFix}
	}
	if len(fixes) > maxPassageEdits {
		return result{}, fmt.Errorf("%w: \"fixes\" holds at most %d fixes a call", errBadArguments, maxPassageEdits)
	}
	if len(fixes) == 1 {
		return e.propose(ctx, s, fixes[0].proposal())
	}
	var b strings.Builder
	refused := 0
	for i, fix := range fixes {
		out, err := e.propose(ctx, s, fix.proposal())
		st, said := out.step, out.text
		st.OK = err == nil
		if err != nil {
			refused++
			said = "NOT suggested: " + refusal(err)
			st = Step{Tool: toolArticleEdit, Summary: toolArticleEdit + ": " + refusal(err)}
		}
		e.addStep(ctx, s, st)
		fmt.Fprintf(&b, "Fix %d (%s): %s\n", i+1, truncate(strings.TrimSpace(fix.Title), 80), said)
	}
	if refused > 0 {
		b.WriteString("Send again only the fixes that were not suggested, corrected, all in one call.")
	}
	return result{text: strings.TrimSpace(b.String()), quiet: true}, nil
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
