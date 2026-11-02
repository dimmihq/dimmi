/*
|--------------------------------------------------------------------------
| Readable Content
|--------------------------------------------------------------------------
|
| A crawled page is mostly navigation, footers, cookie banners and ads.
| Readability finds the main content the way a browser's reader view
| does, so only what the page is about gets stored and indexed. Relative
| links and images inside it are made absolute against the page URL.
|
*/

package dom

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	readability "codeberg.org/readeck/go-readability/v2"
)

/*
ErrNoContent means the page has no main content to extract.
*/
var ErrNoContent = errors.New("dom: no readable content")

/*
Page is the main content of a page. HTML is the cleaned content and
Text its plain text; the other fields come from the page's metadata and
are empty when the page does not say.
*/
type Page struct {
	Title    string
	Byline   string
	Excerpt  string
	SiteName string
	Language string
	HTML     string
	Text     string
}

/*
Readable extracts the main content of the page at pageURL. It returns
ErrNoContent when there is none.
*/
func Readable(html, pageURL string) (Page, error) {
	base, err := url.Parse(pageURL)
	if err != nil {
		return Page{}, fmt.Errorf("parse page url: %w", err)
	}

	page, err := readability.FromReader(strings.NewReader(html), base)
	if err != nil {
		return Page{}, fmt.Errorf("extract readable content: %w", err)
	}

	if page.Node == nil {
		return Page{}, ErrNoContent
	}

	var content, text strings.Builder
	if err := errors.Join(page.RenderHTML(&content), page.RenderText(&text)); err != nil {
		return Page{}, fmt.Errorf("render readable content: %w", err)
	}

	return Page{
		Title:    page.Title(),
		Byline:   page.Byline(),
		Excerpt:  page.Excerpt(),
		SiteName: page.SiteName(),
		Language: page.Language(),
		HTML:     content.String(),
		Text:     text.String(),
	}, nil
}
