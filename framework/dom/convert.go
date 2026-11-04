/*
|--------------------------------------------------------------------------
| HTML Conversion
|--------------------------------------------------------------------------
|
| HTML from email and the web turned into the two forms the application
| stores and searches: plain text, for previews, search and language
| models, and Markdown, the canonical form of written content. Scripts,
| styles and other markup that carries no text are dropped on the way.
|
*/

package dom

import (
	"fmt"

	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/base"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/commonmark"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/table"
	"github.com/k3a/html2text"
)

/*
ToText returns the readable text of HTML: entities decoded, block
elements on their own lines, list items prefixed with " - ", and each
link as its text followed by its URL in angle brackets.
*/
func ToText(html string) string {
	return html2text.HTML2TextWithOptions(html,
		html2text.WithUnixLineBreaks(),
		html2text.WithListSupport(),
		html2text.WithLinksInnerText(),
	)
}

/*
ToMarkdown converts HTML to Markdown. Headings, emphasis, lists, links,
images, quotes and code survive, and data tables become GFM tables, so
cells and rows stay apart; a table used only for layout gives way to
its content. Two lists in a row get no marker between them, as it
would show as text wherever the Markdown is read. Elements without a
Markdown form keep only their text.
Link targets are copied as they are, so the result is as untrusted as
the HTML and is rendered only through a renderer that checks URLs.
*/
func ToMarkdown(html string) (string, error) {
	convert := converter.NewConverter(converter.WithPlugins(
		base.NewBasePlugin(),
		commonmark.NewCommonmarkPlugin(commonmark.WithListEndComment(false)),
		table.NewTablePlugin(
			table.WithCellPaddingBehavior(table.CellPaddingBehaviorMinimal),
			table.WithSkipEmptyRows(true),
			table.WithHeaderPromotion(true),
		),
	))

	out, err := convert.ConvertString(html)
	if err != nil {
		return "", fmt.Errorf("convert html to markdown: %w", err)
	}

	return out, nil
}
