/*
Package articlehtml turns help center articles from other tools into
article Markdown.
*/
package articlehtml

/*
|--------------------------------------------------------------------------
| Article Conversion
|--------------------------------------------------------------------------
|
| Help center articles from other tools turned into dimmi's article
| Markdown, keeping the blocks plain Markdown has no syntax for. Each
| block is read from the HTML Intercom's Articles API returns:
|
|   - a button, <a class="intercom-h2b-button">, becomes ::button, centered
|     when it sits in <div class="intercom-align-center">
|   - a video, an <iframe>, becomes ::embed with its address
|   - a picture in <div class="intercom-align-center"> keeps its place
|   - a callout, <div class="intercom-interblocks-callout">, becomes the
|     alert its background color is closest to
|   - a collapsible section, <details> with a <summary>, becomes
|     :::details
|   - a <figure> keeps its <figcaption> as the picture's caption
|   - a paragraph or heading with class "intercom-align-center" goes into
|     a :::center block with its centered neighbors
|
| dimmi's own article HTML converts back the same way, so an article
| survives a round trip.
|
| The button, video and alignment shapes are documented by Intercom. The
| callout's was seen in a live article and the collapsible section's is
| not documented at all; both stay unverified until a real export is
| read through this converter.
|
*/

import "github.com/dimmihq/dimmi/framework/dom"

/*
intercom is the class names Intercom marks its article blocks with.
*/
func intercom() dom.BlockClasses {
	return dom.BlockClasses{
		Button:  "intercom-h2b-button",
		Center:  "intercom-align-center",
		Callout: "intercom-interblocks-callout",
	}
}

/*
ToMarkdown converts an article's HTML, from Intercom or from dimmi's own
renderer, to article Markdown. The result is as untrusted as the HTML;
the article renderer checks every address.
*/
func ToMarkdown(body string) (string, error) {
	return dom.ToBlocksMarkdown(body, intercom())
}
