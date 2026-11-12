/*
|--------------------------------------------------------------------------
| Counting Help Center Reads
|--------------------------------------------------------------------------
|
| A real help center counts an article page each time a person opens it
| and each search with whether it found anything. Crawlers and the pages
| browsers fetch ahead of a click are not people reading, so they are
| left out. Only counts are kept, never who read.
|
*/

package web

import (
	"strings"

	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/actions/helpcenters"
)

/*
countView counts a view of the real help center's article with the slug.
*/
func (c *HelpCenterController) countView(r *fhttp.Request, lib HelpLibrary, slug string) {
	s, ok := lib.(*site)
	if !ok || c.Analytics == nil || !personReading(r) {
		return
	}
	if at := s.find(slug); at >= 0 {
		c.Analytics.Viewed(r.Context(), s.snap.HelpCenter, s.snap.Articles[at].ID, helpcenters.ViewOnWeb)
	}
}

/*
countSearch counts a search of the real help center.
*/
func (c *HelpCenterController) countSearch(r *fhttp.Request, lib HelpLibrary, query string, results int) {
	s, ok := lib.(*site)
	if !ok || c.Analytics == nil || query == "" || !personReading(r) {
		return
	}
	c.Analytics.Searched(r.Context(), s.snap.HelpCenter, query, results)
}

/*
personReading reports whether the request looks like a person opening
the page rather than a crawler or a fetch ahead of a click.
*/
func personReading(r *fhttp.Request) bool {
	for _, h := range []string{"Sec-Purpose", "Purpose", "X-Purpose", "X-Sec-Purpose", "X-Moz"} {
		if strings.Contains(strings.ToLower(r.Header.Get(h)), "prefetch") {
			return false
		}
	}
	agent := strings.ToLower(r.UserAgent())
	if agent == "" {
		return false
	}
	for _, crawler := range []string{"bot", "crawl", "spider", "slurp", "preview", "headless", "curl", "wget"} {
		if strings.Contains(agent, crawler) {
			return false
		}
	}
	return true
}
