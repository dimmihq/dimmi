/*
|--------------------------------------------------------------------------
| Front Matter
|--------------------------------------------------------------------------
|
| Documents synced from a repository or uploaded as files carry their
| metadata in a front matter block: YAML between "---" lines or TOML
| between "+++" lines, at the very top. The block is size-capped before
| it is decoded, because YAML in particular can expand a few kilobytes
| into a very large value.
|
*/

package markdown

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"go.abhg.dev/goldmark/frontmatter"
)

/*
DefaultFrontMatterBytes is the front matter size limit used when the
caller gives none.
*/
const DefaultFrontMatterBytes = 16 << 10

/*
ErrFrontMatterTooLarge means the front matter block is over the limit.
*/
var ErrFrontMatterTooLarge = errors.New("markdown: front matter too large")

/*
FrontMatter splits a document into its front matter and its Markdown
body. A block counts only if the first line is three or more "-" or "+"
and a later line repeats it exactly; otherwise the whole document is
the body and the metadata is empty. A block longer than maxBytes, or
DefaultFrontMatterBytes when maxBytes is not positive, fails with
ErrFrontMatterTooLarge.
*/
func FrontMatter(src []byte, maxBytes int) (map[string]any, []byte, error) {
	if maxBytes <= 0 {
		maxBytes = DefaultFrontMatterBytes
	}

	block, found := splitFrontMatter(src)
	if !found {
		return map[string]any{}, src, nil
	}

	if len(block.raw) > maxBytes {
		return nil, nil, ErrFrontMatterTooLarge
	}

	meta := map[string]any{}
	if err := block.format.Unmarshal(block.raw, &meta); err != nil {
		return nil, nil, fmt.Errorf("decode %s front matter: %w", strings.ToLower(block.format.Name), err)
	}

	if meta == nil {
		meta = map[string]any{}
	}

	return meta, block.body, nil
}

/*
frontMatterBlock is a front matter block found at the top of a
document: its format, its contents between the fences, and the body
after it.
*/
type frontMatterBlock struct {
	format frontmatter.Format
	raw    []byte
	body   []byte
}

/*
splitFrontMatter finds the front matter block at the top of a document.
*/
func splitFrontMatter(src []byte) (frontMatterBlock, bool) {
	first, rest, ok := bytes.Cut(src, []byte("\n"))
	if !ok {
		return frontMatterBlock{}, false
	}

	fence := bytes.TrimRight(first, " \t\r")

	format, ok := fenceFormat(fence)
	if !ok {
		return frontMatterBlock{}, false
	}

	for offset := 0; offset < len(rest); {
		line, _, _ := bytes.Cut(rest[offset:], []byte("\n"))
		if bytes.Equal(bytes.TrimRight(line, " \t\r"), fence) {
			body := rest[min(offset+len(line)+1, len(rest)):]

			return frontMatterBlock{format: format, raw: rest[:offset], body: body}, true
		}

		offset += len(line) + 1
	}

	return frontMatterBlock{}, false
}

/*
fenceFormat returns the format a fence line opens: three or more of the
same delimiter character and nothing else.
*/
func fenceFormat(fence []byte) (frontmatter.Format, bool) {
	if len(fence) < 3 || len(bytes.Trim(fence, string(fence[:1]))) != 0 {
		return frontmatter.Format{}, false
	}

	for _, format := range frontmatter.DefaultFormats {
		if format.Delim == fence[0] {
			return format, true
		}
	}

	return frontmatter.Format{}, false
}
