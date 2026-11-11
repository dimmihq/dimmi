/*
|--------------------------------------------------------------------------
| Notion Blocks
|--------------------------------------------------------------------------
|
| A Notion page is a tree of blocks. Text blocks become their Markdown
| twins with bold, italic, code and links kept; lists, to-dos and
| toggles nest; tables become GFM tables; pictures are collected so an
| import can copy them. Child pages are documents of their own and are
| skipped here; blocks Markdown has no form for keep only their text.
|
*/

package notion

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/app/integrations/knowledge/sources"
)

/*
maxDepth is how deep nested blocks are read.
*/
const maxDepth = 3

/*
page is a Notion page as the API returns it.
*/
type page struct {
	ID         string              `json:"id"`
	URL        string              `json:"url"`
	PublicURL  string              `json:"public_url"`
	Archived   bool                `json:"archived"`
	InTrash    bool                `json:"in_trash"`
	LastEdited time.Time           `json:"last_edited_time"`
	Parent     parent              `json:"parent"`
	Properties map[string]property `json:"properties"`
}

/*
parent is what a page sits under.
*/
type parent struct {
	Type   string `json:"type"`
	PageID string `json:"page_id"`
}

/*
property is one property of a page; only titles are read.
*/
type property struct {
	Type  string     `json:"type"`
	Title []richText `json:"title"`
}

/*
title is the page's title property.
*/
func (p page) title() string {
	for _, prop := range p.Properties {
		if prop.Type == "title" {
			if t := strings.TrimSpace(plain(prop.Title)); t != "" {
				return t
			}
		}
	}

	return "Untitled"
}

/*
richText is a run of styled text.
*/
type richText struct {
	PlainText   string `json:"plain_text"`
	Href        string `json:"href"`
	Annotations struct {
		Bold   bool `json:"bold"`
		Italic bool `json:"italic"`
		Strike bool `json:"strikethrough"`
		Code   bool `json:"code"`
	} `json:"annotations"`
}

/*
block is one Notion block; its content sits under a key named after
its type.
*/
type block struct {
	ID          string
	Type        string
	HasChildren bool
	body        blockContent
}

/*
blockContent is the shared shape of a block's content.
*/
type blockContent struct {
	RichText []richText   `json:"rich_text"`
	Checked  bool         `json:"checked"`
	Language string       `json:"language"`
	Caption  []richText   `json:"caption"`
	Cells    [][]richText `json:"cells"`
	URL      string       `json:"url"`
	Type     string       `json:"type"`
	External struct {
		URL string `json:"url"`
	} `json:"external"`
	File struct {
		URL string `json:"url"`
	} `json:"file"`
}

/*
blocks renders the page's blocks, and those nested under them, as
Markdown.
*/
func (d *Driver) blocks(ctx context.Context, s settings, id string, depth int) (string, []sources.Asset, error) {
	children, err := d.children(ctx, s, id)
	if err != nil {
		return "", nil, err
	}

	w := &writer{}
	number := 0

	for i, child := range children {
		number = nextNumber(child.Type, number)

		if i > 0 && listItem(children[i-1].Type) && !listItem(child.Type) {
			w.b.WriteString("\n")
		}

		if err := d.block(ctx, s, w, placed{block: child, depth: depth, number: number}); err != nil {
			return "", nil, err
		}
	}

	return w.b.String(), w.assets, nil
}

/*
writer gathers a page's Markdown and pictures.
*/
type writer struct {
	b      strings.Builder
	assets []sources.Asset
}

/*
placed is a block with how deep it is nested and, in a numbered list,
its number.
*/
type placed struct {
	block
	depth  int
	number int
}

/*
block writes one block and the blocks nested under it.
*/
func (d *Driver) block(ctx context.Context, s settings, w *writer, p placed) error {
	line, asset := render(p.block, p.number)
	if asset != nil {
		w.assets = append(w.assets, *asset)
	}

	indent := strings.Repeat("  ", p.depth)
	if line != "" {
		w.b.WriteString(indent + strings.ReplaceAll(line, "\n", "\n"+indent) + "\n")
	}

	if p.Type == "table" {
		w.b.WriteString(d.table(ctx, s, p.ID, indent))
	}

	if p.HasChildren && p.Type != "child_page" && p.Type != "table" && p.depth < maxDepth {
		nested, more, err := d.blocks(ctx, s, p.ID, p.depth+1)
		if err != nil {
			return err
		}

		w.b.WriteString(nested)
		w.assets = append(w.assets, more...)
	}

	if !listItem(p.Type) {
		w.b.WriteString("\n")
	}

	return nil
}

/*
children reads every block directly under a block or page.
*/
func (d *Driver) children(ctx context.Context, s settings, id string) ([]block, error) {
	var out []block

	cursor := ""

	for range 50 {
		path := "/blocks/" + url.PathEscape(id) + "/children?page_size=100"
		if cursor != "" {
			path += "&start_cursor=" + url.QueryEscape(cursor)
		}

		var resp struct {
			Results    []map[string]json.RawMessage `json:"results"`
			HasMore    bool                         `json:"has_more"`
			NextCursor string                       `json:"next_cursor"`
		}

		if err := d.call(ctx, s, request{method: "GET", path: path, out: &resp}); err != nil {
			return nil, err
		}

		for _, r := range resp.Results {
			out = append(out, decodeBlock(r))
		}

		if !resp.HasMore {
			break
		}

		cursor = resp.NextCursor
	}

	return out, nil
}

/*
table renders a table block's rows as a GFM table, its first row the
header.
*/
func (d *Driver) table(ctx context.Context, s settings, id, indent string) string {
	rows, err := d.children(ctx, s, id)
	if err != nil {
		return ""
	}

	var b strings.Builder

	for i, row := range rows {
		c := row.body
		cells := make([]string, len(c.Cells))

		for j, cell := range c.Cells {
			cells[j] = strings.ReplaceAll(inline(cell), "|", `\|`)
		}

		b.WriteString(indent + "| " + strings.Join(cells, " | ") + " |\n")

		if i == 0 {
			b.WriteString(indent + "|" + strings.Repeat(" --- |", len(cells)) + "\n")
		}
	}

	return b.String()
}

func nextNumber(kind string, number int) int {
	if kind == "numbered_list_item" {
		return number + 1
	}

	return 0
}

func listItem(kind string) bool {
	return kind == "bulleted_list_item" || kind == "numbered_list_item" || kind == "to_do"
}

/*
errorsIsNotFound reports whether Notion said a page is gone or not
shared.
*/
func errorsIsNotFound(err error) bool {
	return errors.Is(err, sources.ErrNotFound) || errors.Is(err, sources.ErrUnauthorized)
}
