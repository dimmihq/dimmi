/*
|--------------------------------------------------------------------------
| Help Center Pages
|--------------------------------------------------------------------------
|
| Each page of the help center and its portal: what it says about itself
| and the variables its template reads. The markup is the theme's; the
| default theme's templates live in resources/themes/default.
|
*/

package helpcenter

import (
	"github.com/dimmihq/dimmi/framework/liquid"
	"github.com/dimmihq/dimmi/framework/view"
)

/*
Home is the help center's front page.
*/
func Home(c Center, collections []Collection, popular []Summary, updates []Update) view.Component {
	m := Meta{
		Title: c.Name, Description: c.Tagline, Section: "help",
		Trail: []Crumb{{Title: c.T("help_center"), Path: "/"}}, Markdown: "/index.md",
	}
	return Render(c, "home", m, map[string]any{
		"collections": c.collections(collections), "popular": c.summaries(popular), "updates": c.updates(updates),
	})
}

/*
CollectionPage lists a collection, its sub-collections and articles.
*/
func CollectionPage(c Center, all []Collection, col Collection, trail []Crumb) view.Component {
	crumbs := helpTrail(c, trail, col.Title, "")
	m := Meta{
		Title: col.Title + " | " + c.Name, Description: col.Description, Section: "help", Trail: crumbs,
		Markdown: "/collections/" + col.Slug + ".md",
	}
	v := c.collection(col)
	inside := c.collection(Collection{
		Slug: col.Slug, Title: c.T("in_this_collection"), Children: col.Children, Articles: col.Articles,
	})
	return Render(c, "collection", m, map[string]any{
		"collections": c.collections(all), "breadcrumb": c.crumbs(crumbs),
		"collection": liquid.Value(v), "in_collection": liquid.Value(inside),
	})
}

/*
ArticlePage shows an article with whether it helped.
*/
func ArticlePage(c Center, all []Collection, a Article, helpful Helpful) view.Component {
	m := Meta{
		Title: a.Title + " | " + c.Name, Description: a.Description, Section: "help",
		Trail: helpTrail(c, a.Trail, "", a.Title), Markdown: "/articles/" + a.Slug + ".md",
		NoIndex: a.Untranslated != "",
	}
	return Render(c, "article", m, map[string]any{
		"collections": c.collections(all), "article": c.article(all, a, helpful),
		"breadcrumb": c.crumbs(helpTrail(c, a.Trail, "", "")),
	})
}

/*
SearchPage shows what a question found.
*/
func SearchPage(c Center, all []Collection, query string, results []Result) view.Component {
	m := Meta{
		Title: searchTitle(c, query) + " | " + c.Name, Section: "help",
		Trail:   []Crumb{{Title: c.T("help_center"), Path: "/"}, {Title: c.T("search"), Path: "/search"}},
		NoIndex: true,
	}
	found := make([]ResultVars, 0, len(results))
	for _, r := range results {
		found = append(
			found,
			ResultVars{Kind: r.Kind, Title: r.Title, URL: c.url(r.Path), Excerpt: r.Excerpt, Trail: r.Trail},
		)
	}
	return Render(c, "search", m, map[string]any{
		"collections": c.collections(all), "search": SearchVars{Query: query, Results: found},
	})
}

/*
NotFoundPage answers a path the portal has nothing at.
*/
func NotFoundPage(c Center, all []Collection, popular []Summary) view.Component {
	m := Meta{
		Title: c.T("page_not_found") + " | " + c.Name, Section: "help",
		Trail:   []Crumb{{Title: c.T("help_center"), Path: "/"}, {Title: c.T("not_found"), Path: "/"}},
		NoIndex: true,
	}
	return Render(c, "404", m, map[string]any{"collections": c.collections(all), "popular": c.summaries(popular)})
}

/*
ReaderForm is what a reader's sign-in pages carry: where each form goes,
the page to return to, the team's login address when the help center
takes it, the address typed, how it is shown once a code went to it, and
what went wrong, if anything.
*/
type ReaderForm struct {
	// Where the email form posts.
	Action string
	// Where the code form posts.
	CheckURL string
	// Where the form that sends another code posts.
	ResendURL string
	// Back to typing another address.
	RestartURL string
	// The page to return to after signing in.
	Next string
	// The team's own login, empty when the help center takes only email.
	ProductURL string
	// The address typed.
	Email string
	// The address as the code page shows it, such as a**@example.com.
	Masked string
	// What went wrong, empty for nothing.
	Error string
}

/*
ReaderSignInPage takes a reader back from the team's product, or in
from the messenger.
*/
func ReaderSignInPage(c Center, action, next string) view.Component {
	m := Meta{Title: c.T("signing_in") + " | " + c.Product, NoIndex: true}
	return Render(c, "sign_in_return", m, map[string]any{"sign_in": ReaderForm{Action: action, Next: next}})
}

/*
ReaderEmailPage lets a reader sign in with their email.
*/
func ReaderEmailPage(c Center, f ReaderForm) view.Component {
	return Render(
		c, "sign_in", Meta{Title: c.T("sign_in") + " | " + c.Name, NoIndex: true},
		map[string]any{"sign_in": f},
	)
}

/*
ReaderCodePage takes the code mailed to the reader.
*/
func ReaderCodePage(c Center, f ReaderForm) view.Component {
	return Render(
		c, "sign_in_code", Meta{Title: c.T("check_email") + " | " + c.Name, NoIndex: true},
		map[string]any{"sign_in": f},
	)
}
