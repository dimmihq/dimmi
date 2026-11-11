/*
|--------------------------------------------------------------------------
| Listings
|--------------------------------------------------------------------------
|
| A listing names a source's documents a page at a time, each with the
| version that tells the engine whether it changed, so only changed
| documents are read again. A connection check says what it reached.
|
*/

package sources

import "time"

/*
Ref is one document a listing found. ExternalID names it for the
driver and never changes; URL is where it lives, Path how include and
exclude patterns see it, and Title its name when the listing knows it.
Version changes whenever the document does, such as an update time or
a content hash; an empty version always fetches again. Document, when
the listing already read it, as a crawl service does, is stored as it
is instead of being fetched again.
*/
type Ref struct {
	ExternalID string
	URL        string
	Path       string
	Title      string
	Version    string
	UpdatedAt  time.Time
	Document   *Document
}

/*
Page is one page of a listing, the cursor of the next one, empty at
the end, and what reading it used at a paid service.
*/
type Page struct {
	Refs  []Ref
	Next  string
	Usage Usage
}

/*
Usage is what a listing used at a service billed by time, such as a
hosted browser: the service and how long it ran, in milliseconds.
*/
type Usage struct {
	Service   string
	BrowserMs int64
}

/*
CheckResult is what a working connection says about itself: the
account it reaches and, when known cheaply, how many documents it has.
*/
type CheckResult struct {
	Account  string `json:"account"`
	Estimate int    `json:"estimate"`
}
