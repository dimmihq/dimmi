/*
|--------------------------------------------------------------------------
| Changelog Pages
|--------------------------------------------------------------------------
|
| What the public changelog's pages show. The changelog lives on its help
| center's host under /changelog and wears the help center's header,
| brand and footer, so every page takes the help center along with the
| entries. Bodies arrive rendered and sanitized; paths are already under
| the help center's base.
|
*/

package changelog

import (
	"time"

	"github.com/dimmihq/dimmi/resources/views/helpcenter"
)

/*
Label is a kind of change an entry is filed under, such as New or
Fixed, with the hex color its dot is drawn in and the path of its own
filtered page.
*/
type Label struct {
	Name  string
	Slug  string
	Color string
	Path  string
	Count int
}

/*
Reaction is one of the fixed emoji visitors react with and how many
have.
*/
type Reaction struct {
	Emoji string
	Name  string
	Count int
}

/*
Entry is a published change. Cover is the path of its image, empty when
it has none. Full says whether a list shows its whole body or only its
summary with a link to read on. An entry without Reactions shows no
reaction buttons.
*/
type Entry struct {
	Slug      string
	Path      string
	Title     string
	Summary   string
	Body      string
	Labels    []Label
	Version   string
	Cover     string
	Published time.Time
	Authors   []helpcenter.Person
	Reactions []Reaction
	Related   []helpcenter.Summary
	Full      bool
}

/*
Index is a page of the changelog: its entries newest first, the labels
to filter by with the one being shown, the paths of the newer and older
pages when there are any, and the year's weeks for the release strip.
Subscribed is set after a visitor asked to subscribe, to tell them to
check their inbox. Title heads the page, Changelog when empty, and
NoSubscribe hides the subscribe form of a changelog that takes no
subscriptions.
*/
type Index struct {
	Title       string
	NoSubscribe bool
	Entries     []Entry
	Labels      []Label
	Active      string
	Newer       string
	Older       string
	Year        int
	Weeks       []Week
	Subscribed  bool
}
