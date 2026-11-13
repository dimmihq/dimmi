/*
|--------------------------------------------------------------------------
| Help Center Articles
|--------------------------------------------------------------------------
|
| The help center is the agent's knowledge, so it is written the way a
| real support team writes one: a few dozen articles by hand on billing,
| accounts, getting started and returns, and families of articles built
| from one hand-written template per kind, filled with the facts of
| each carrier, integration, API operation and error code. A family
| article reads like its siblings, as real ones do, but every fact in it
| is its own. The longest are the API reference pages.
|
*/

package stowline

import (
	"fmt"
	"regexp"
	"strings"
)

/*
Collection is one section of the help center.
*/
type Collection struct {
	Slug, Name, Description, Icon string
}

/*
Collections returns the help center's sections in the order the help
center shows them.
*/
func Collections() []Collection {
	return []Collection{
		{"getting-started", "Getting started", "Set up your account, connect a store and ship your first order.", "rocket"},
		{"shipping-labels", "Shipping labels", "Rates, labels, packing slips, batches and printing.", "tag"},
		{"carriers", "Carriers", "Connect carrier accounts, services, pickups and carrier rules.", "truck"},
		{"integrations", "Integrations", "Stores, marketplaces, ERPs and accounting tools.", "plug"},
		{"tracking-returns", "Tracking and returns", "Tracking pages, notifications and the returns portal.", "rotate"},
		{"billing", "Billing and plans", "Plans, invoices, payment methods and label charges.", "credit-card"},
		{"account-security", "Account and security", "Teammates, roles, single sign-on and data.", "shield"},
		{"api", "API and webhooks", "Build on Stowline: authentication, endpoints and events.", "code"},
		{"troubleshooting", "Troubleshooting", "Error messages and what to do about them.", "wrench"},
	}
}

/*
Article is one help center article: its section, title, address,
summary, Markdown body and who may read it.
*/
type Article struct {
	Collection  string
	Title       string
	Slug        string
	Description string
	Body        string
	Visibility  string
}

/*
Articles returns every article of the help center, hand-written ones
first, each family after.
*/
func Articles() []Article {
	var out []Article
	out = append(out, gettingStartedArticles()...)
	out = append(out, labelArticles()...)
	out = append(out, billingArticles()...)
	out = append(out, accountArticles()...)
	out = append(out, returnsArticles()...)
	out = append(out, internalArticles()...)

	for _, c := range Carriers() {
		out = append(out, carrierArticles(c)...)
	}
	for _, p := range Platforms() {
		out = append(out, platformArticles(p)...)
	}
	out = append(out, apiGuideArticles()...)
	out = append(out, referenceArticles()...)
	out = append(out, guideArticles()...)
	out = append(out, releaseArticles()...)
	for i, c := range Carriers() {
		out = append(out, carrierGuide(i, c))
	}
	for i := range errorAreas() {
		out = append(out, errorReference(i))
	}
	out = append(out, serviceArticles()...)
	out = append(out, sectionArticles()...)
	out = append(out, completeReferences(out)...)
	out = append(out, quickstartArticle())
	for _, e := range ErrorCodes() {
		out = append(out, errorArticle(e))
	}

	taken := map[string]int{}
	for i := range out {
		if out[i].Slug == "" {
			out[i].Slug = Slugify(out[i].Title)
		}
		taken[out[i].Slug]++
		if taken[out[i].Slug] > 1 {
			out[i].Slug = fmt.Sprintf("%s-%d", out[i].Slug, taken[out[i].Slug])
		}
		if out[i].Visibility == "" {
			out[i].Visibility = "public"
		}
	}
	out = withQuestions(out)
	for i := range out {
		out[i].Body = strings.TrimSpace(out[i].Body) + "\n"
	}
	return out
}

/*
WordCount counts an article's words the way a reader would, code and
tables included.
*/
func WordCount(body string) int {
	return len(strings.Fields(body))
}

var notSlug = regexp.MustCompile(`[^a-z0-9]+`)

/*
Slugify turns a title into an article address: lower case, dashes
between words, at most 120 characters.
*/
func Slugify(title string) string {
	s := strings.Trim(notSlug.ReplaceAllString(strings.ToLower(title), "-"), "-")
	if len(s) > 120 {
		s = strings.Trim(s[:120], "-")
	}
	return s
}

/*
list renders items as a Markdown bullet list.
*/
func list(items ...string) string {
	var b strings.Builder
	for _, it := range items {
		b.WriteString("- " + it + "\n")
	}
	return b.String()
}

/*
steps renders items as a numbered Markdown list.
*/
func steps(items ...string) string {
	var b strings.Builder
	for i, it := range items {
		fmt.Fprintf(&b, "%d. %s\n", i+1, it)
	}
	return b.String()
}

/*
table renders a Markdown table from a header row and rows.
*/
func table(head []string, rows [][]string) string {
	var b strings.Builder
	b.WriteString("| " + strings.Join(head, " | ") + " |\n|")
	for range head {
		b.WriteString(" --- |")
	}
	b.WriteString("\n")
	for _, r := range rows {
		b.WriteString("| " + strings.Join(r, " | ") + " |\n")
	}
	return b.String()
}

/*
code renders a fenced code block.
*/
func code(lang, body string) string {
	return "```" + lang + "\n" + strings.TrimSpace(body) + "\n```\n"
}
