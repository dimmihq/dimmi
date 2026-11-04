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
	"github.com/jhillyerd/enmime/v2"
	"golang.org/x/net/html/charset"
)

/*
walk reads the header fields from the start of the message, then the
rest of it as a stream.
*/
func walk(head []byte, rest io.Reader, limits Limits) (*Parsed, error) {
	env, err := enmime.ReadEnvelope(bytes.NewReader(headerBlock(head)))
	if err != nil {
		return nil, fmt.Errorf("read message header: %w", err)
	}

	parsed := fromHeader(env)
	parsed.Truncated = true

	source := &tracked{Reader: rest}
	entity, rootErr := message.Read(io.MultiReader(bytes.NewReader(head), source))

	if entity != nil {
		rootErr = entity.Walk(func(path []int, part *message.Entity, err error) error {
			if path == nil {
				err = cmp.Or(err, rootErr)
			}

			return parsed.visit(part, limits, err)
		})
	}

	if source.err != nil {
		return nil, fmt.Errorf("read message: %w", source.err)
	}

	if rootErr != nil {
		parsed.Warnings = append(parsed.Warnings, "unreadable structure: "+rootErr.Error())
	}

	return parsed, nil
}

/*
visit adds a part to the text or HTML body when it is one, and lists it
as a dropped attachment otherwise.
*/
func (p *Parsed) visit(part *message.Entity, limits Limits, partErr error) error {
	mediaType, params, _ := part.Header.ContentType()
	if strings.HasPrefix(mediaType, "multipart/") {
		return nil
	}

	body := part.Body
	if message.IsUnknownCharset(partErr) {
		body = decodeCharset(body, params["charset"])
	}

	disposition, dispositionParams, _ := part.Header.ContentDisposition()
	filename := cmp.Or(dispositionParams["filename"], params["name"])

	if filename == "" && disposition != "attachment" {
		switch mediaType {
		case typePlain:
			return keep(&p.Text, textSeparator, body, limits.MaxPartBytes)
		case typeHTML:
			return keep(&p.HTML, htmlSeparator, body, limits.MaxPartBytes)
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
keep adds a body to what was kept, up to the cap in all, and skips the
rest of it.
*/
func keep(target *string, separator string, body io.Reader, maxBytes int64) error {
	room := max(maxBytes-int64(len(*target)), 0)

	kept, err := io.ReadAll(io.LimitReader(body, room))
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}

	if *target != "" && len(kept) > 0 {
		*target += separator
	}

	*target += string(kept)

	if _, err := io.Copy(io.Discard, body); err != nil {
		return fmt.Errorf("read body: %w", err)
	}

	return nil
}

/*
decodeCharset converts a body from its declared charset. An unknown
label leaves the bytes as they are, to be made valid UTF-8 later.
*/
func decodeCharset(body io.Reader, label string) io.Reader {
	decoded, err := charset.NewReaderLabel(label, body)
	if err != nil {
		return body
	}

	return decoded
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
