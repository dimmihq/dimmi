/*
|--------------------------------------------------------------------------
| Documents
|--------------------------------------------------------------------------
|
| What a driver reads is one document: its Markdown, where it sits at
| its source, its pictures, its language and who may see it. The engine
| needs nothing else to index it or turn it into an article.
|
*/

package sources

import "time"

/*
Collection is one level of the place a document sits in at its source,
such as a help center collection or a folder.
*/
type Collection struct {
	ExternalID  string `json:"external_id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	URL         string `json:"url,omitempty"`
}

/*
Asset is a picture a document shows, by the address it is read from.
*/
type Asset struct {
	URL  string `json:"url"`
	Name string `json:"name,omitempty"`
}

/*
Visibility says whether a document may be shown and linked to
customers, or only read by the agent.
*/
type Visibility string

/*
The visibilities.
*/
const (
	VisibilityPublic   Visibility = "public"
	VisibilityInternal Visibility = "agent_only"
)

/*
Document is what a fetch read. Markdown is its normalized body and
Collections the place it sits in, outermost first. Locale is its
language, and TranslationOf the external id of the document it
translates, empty for an original. Draft marks one not published at
its source. Links are more documents worth discovering, for drivers
that crawl. Version, when set, replaces the version the listing gave.
*/
type Document struct {
	Title         string
	Description   string
	Markdown      string
	URL           string
	Path          string
	Locale        string
	TranslationOf string
	Collections   []Collection
	Assets        []Asset
	Visibility    Visibility
	Draft         bool
	Links         []string
	Version       string
	UpdatedAt     time.Time
}
