/*
|--------------------------------------------------------------------------
| Oversized Messages
|--------------------------------------------------------------------------
|
| A message over the size limit is never held whole. It is read once as
| a stream: its text and HTML bodies are kept up to a cap each, in the
| order they were written, and every other part is counted and listed as
| a dropped attachment, so the reader still learns what was sent and what
| could not be kept.
|
| A broken structure ends the walk with a warning, keeping what was read
| so far. Failing to read the source itself is an error, so the caller
| can try again.
|
*/

package parse

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/emersion/go-message"
)

/*
walk reads the header fields from the start of the message, then the
rest of it as a stream. A body that yields nothing is kept as plain text,
up to the cap.
*/
func walk(first []byte, rest io.Reader, limits Limits) (*Parsed, error) {
	top, ok := readHead(first)
	if !ok {
		top = head{message: first, body: first}
	}

	parsed := fromHeader(top)
	parsed.Truncated = true

	source := &tracked{Reader: rest}
	entity, rootErr := message.Read(io.MultiReader(bytes.NewReader(top.message), source))

	if entity != nil {
		rootErr = entity.Walk(func(_ []int, part *message.Entity, _ error) error {
			return parsed.visit(part, limits)
		})
	}

	if source.err != nil {
		return nil, fmt.Errorf("read message: %w", source.err)
	}

	if rootErr != nil {
		parsed.Warnings = append(parsed.Warnings, "unreadable structure: "+rootErr.Error())
	}

	if parsed.empty() {
		parsed.Text = decodeText(top.body[:min(int64(len(top.body)), limits.MaxPartBytes)], "")
	}

	return parsed, nil
}

/*
visit adds a part to the text or HTML body when it is one, and lists it
as a dropped attachment otherwise.
*/
func (p *Parsed) visit(part *message.Entity, limits Limits) error {
	mediaType, params, _ := part.Header.ContentType()
	if strings.HasPrefix(mediaType, "multipart/") {
		return nil
	}

	body := part.Body

	disposition, dispositionParams, _ := part.Header.ContentDisposition()
	filename := cmp.Or(dispositionParams["filename"], params["name"])

	if filename == "" && disposition != "attachment" {
		switch mediaType {
		case typePlain, typeHTML:
			return p.keep(mediaType, params[charsetParam], body, limits.MaxPartBytes)
		}
	}

	size, err := io.Copy(io.Discard, body)
	if err != nil {
		return fmt.Errorf("read part: %w", err)
	}

	p.Attachments = append(p.Attachments, Attachment{
		Filename:    filename,
		ContentType: mediaType,
		ContentID:   strings.Trim(part.Header.Get("Content-Id"), "<>"),
		Inline:      disposition == "inline",
		Size:        size,
		Dropped:     true,
	})

	return nil
}

/*
keep adds a body to what was kept of its type, up to the cap in all, and
skips the rest of it.
*/
func (p *Parsed) keep(mediaType, label string, body io.Reader, maxBytes int64) error {
	target, separator := &p.Text, textSeparator
	if mediaType == typeHTML {
		target, separator = &p.HTML, htmlSeparator
	}

	kept, err := io.ReadAll(io.LimitReader(body, max(maxBytes-int64(len(*target)), 0)))
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}

	if *target != "" && len(kept) > 0 {
		*target += separator
	}

	*target += decodeBody(kept, label, mediaType)

	if _, err := io.Copy(io.Discard, body); err != nil {
		return fmt.Errorf("read body: %w", err)
	}

	return nil
}

/*
tracked remembers a read failure of the source, which unlike a broken
structure is worth retrying.
*/
type tracked struct {
	io.Reader
	err error
}

/*
Read reads from the source, keeping any failure other than its end.
*/
func (t *tracked) Read(p []byte) (int, error) {
	n, err := t.Reader.Read(p)
	if err != nil && !errors.Is(err, io.EOF) {
		t.err = err
	}

	return n, err
}
