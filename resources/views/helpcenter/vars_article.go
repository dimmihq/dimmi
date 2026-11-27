/*
|--------------------------------------------------------------------------
| Theme Variables: Articles And Search
|--------------------------------------------------------------------------
|
| An article as its page reads it, the changelog entries the home page
| lists and what a search found.
|
*/

package helpcenter

import (
	"net/url"
	"time"

	"github.com/dimmihq/dimmi/framework/liquid"
)

/*
HeadingVars is an entry of an article's table of contents.
*/
type HeadingVars struct {
	// The heading's text.
	Title string
	// Its anchor.
	ID string
	// The headings under it.
	Children []HeadingVars
}

/*
FeedbackVars is where a reader stands with "Was this article helpful?".
*/
type FeedbackVars struct {
	// Yes or no once answered, else empty.
	Answer string
	// They said what was missing after a no.
	Commented bool
	// They sent too much feedback in the last hour.
	Limited bool
	// Where the yes and no form posts, with helpful=yes or no.
	URL string
	// Where the note posts, with comment.
	CommentURL string
}

/*
ArticleVars is article, on the article page.
*/
type ArticleVars struct {
	SummaryVars
	// Its Markdown twin.
	MarkdownURL string
	// The same, as a path, for scripts that fetch it.
	MarkdownPath string
	// Opens ChatGPT asking about the article.
	ChatGPTURL string `liquid:"chatgpt_url"`
	// Opens Claude asking about the article.
	ClaudeURL string
	// Its body, sanitized HTML.
	Body liquid.HTML
	// Its table of contents.
	Contents []HeadingVars
	// Who wrote it; name is empty for nobody.
	Author PersonVars
	// When it last changed.
	Updated time.Time
	// How long ago that is, such as 3 days ago, or the date when unknown.
	Age string
	// Age says how long ago rather than a date.
	AgeKnown bool
	// Its reading time in minutes.
	Minutes int
	// The article before it in its collection, nil for none.
	Previous *SummaryVars
	// The article after it, nil for none.
	Next *SummaryVars
	// Articles related to it.
	Related []SummaryVars
	// The page's language when the article is not translated into it, else empty.
	Untranslated string
	// The language it is written in when shown untranslated, else empty.
	OriginalLang string
	// The slug of the collection it sits in.
	CollectionSlug string
	// That collection, nil for none.
	Collection *CollectionVars
	// Where the reader stands with Was this helpful.
	Feedback FeedbackVars
}

/*
UpdateVars is a changelog entry the home page lists.
*/
type UpdateVars struct {
	// Its title.
	Title string
	// Its address.
	URL string
	// Its first label, empty for none.
	Label string
	// When it was published.
	Published time.Time
}

/*
SearchVars is search, on the search page.
*/
type SearchVars struct {
	// What the reader searched for.
	Query string
	// What it found, best first.
	Results []ResultVars
}

func headings(list []Heading) []HeadingVars {
	out := make([]HeadingVars, 0, len(list))
	for _, h := range list {
		out = append(out, HeadingVars{Title: h.Title, ID: h.ID, Children: headings(h.Children)})
	}
	return out
}

func (c Center) article(all []Collection, a Article, h Helpful) ArticleVars {
	md := Path(c, "/articles/"+a.Slug+".md")
	ask := "Read " + c.Origin + md + " so I can ask questions about it."
	v := ArticleVars{
		SummaryVars: c.summary(a.Summary), MarkdownURL: md, MarkdownPath: md,
		ChatGPTURL: "https://chatgpt.com/?hints=search&q=" + url.QueryEscape(ask),
		ClaudeURL:  "https://claude.ai/new?q=" + url.QueryEscape(ask),
		Body:       liquid.HTML(a.Body), Contents: headings(a.Contents), Author: personVars(a.Author),
		Updated: a.Updated, Age: age(c, a), AgeKnown: a.Age != "", Minutes: a.Minutes,
		Related: c.summaries(a.Related), Untranslated: a.Untranslated, OriginalLang: a.OriginalLang,
		CollectionSlug: trailEnd(a.Trail),
		Feedback: FeedbackVars{
			Answer: h.Answer, Commented: h.Commented, Limited: h.Limited,
			URL:        c.url("/articles/" + a.Slug + "/feedback"),
			CommentURL: c.url("/articles/" + a.Slug + "/feedback/comment"),
		},
	}
	if a.Previous != nil {
		p := c.summary(*a.Previous)
		v.Previous = &p
	}
	if a.Next != nil {
		n := c.summary(*a.Next)
		v.Next = &n
	}
	if col, ok := collectionBySlug(all, v.CollectionSlug); ok {
		cv := c.collection(col)
		v.Collection = &cv
	}
	return v
}

func (c Center) updates(list []Update) []UpdateVars {
	out := make([]UpdateVars, 0, len(list))
	for _, u := range list {
		out = append(out, UpdateVars{Title: u.Title, URL: c.url(u.Path), Label: u.Label, Published: u.Published})
	}
	return out
}

func searchTitle(c Center, query string) string {
	if query == "" {
		return c.T("search")
	}
	return c.T("search_results_for", "query", query)
}
