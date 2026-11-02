/*
|--------------------------------------------------------------------------
| Multipart Reports
|--------------------------------------------------------------------------
|
| Bounces and complaints share one envelope: a multipart/report message
| with a part for people, a machine readable part of header-like field
| blocks, and the original message or just its headers. Servers order
| and nest these parts loosely, so parts are found by type wherever they
| are rather than by position.
|
*/

package report

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/mail"
	"net/textproto"
	"slices"
	"strings"

	"github.com/jhillyerd/enmime/v2"
)

/*
ErrNotReport rejects a message that is not a multipart/report with the
machine readable part asked for.
*/
var ErrNotReport = errors.New("report: not a multipart report")

/*
Report is the machine readable content of a report message. Fields are
its blocks in order. Original holds the header of the reported message,
nil when the report does not include it.
*/
type Report struct {
	Fields   []textproto.MIMEHeader
	Original mail.Header
}

/*
Read finds the part of one of the machine readable types in the report
and reads its field blocks and the original message header.
*/
func Read(raw []byte, types ...string) (*Report, error) {
	root, err := enmime.ReadParts(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrNotReport, err)
	}

	if root.ContentType != "multipart/report" {
		return nil, fmt.Errorf("%w: content type %s", ErrNotReport, root.ContentType)
	}

	machine := root.DepthMatchFirst(func(p *enmime.Part) bool { return slices.Contains(types, p.ContentType) })
	if machine == nil {
		return nil, fmt.Errorf("%w: no %s part", ErrNotReport, strings.Join(types, " or "))
	}

	fields, err := blocks(machine.Content)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrNotReport, err)
	}

	report := &Report{Fields: fields}

	original := root.DepthMatchFirst(func(p *enmime.Part) bool {
		return slices.Contains([]string{"message/rfc822", "text/rfc822-headers", "message/global"}, p.ContentType)
	})
	if original != nil {
		report.Original = header(original.Content)
	}

	return report, nil
}

/*
blocks reads field blocks separated by blank lines, skipping empty ones.
*/
func blocks(data []byte) ([]textproto.MIMEHeader, error) {
	r := textproto.NewReader(bufio.NewReader(bytes.NewReader(append(data, "\r\n\r\n"...))))

	var out []textproto.MIMEHeader
	for {
		block, err := r.ReadMIMEHeader()
		if errors.Is(err, io.EOF) {
			return out, nil
		}

		if err != nil {
			return nil, fmt.Errorf("read fields: %w", err)
		}

		if len(block) > 0 {
			out = append(out, block)
		}
	}
}

/*
header reads the header of an included message, or of a headers only
part, leniently: an unreadable one counts as absent.
*/
func header(content []byte) mail.Header {
	msg, err := mail.ReadMessage(bytes.NewReader(append(content, "\r\n\r\n"...)))
	if err != nil {
		return nil
	}

	return msg.Header
}

/*
Value returns a field value after its type label, as in
"rfc822; ana@example.com" or "smtp; 550 5.1.1 User unknown".
*/
func Value(field string) string {
	if _, value, ok := strings.Cut(field, ";"); ok {
		return strings.TrimSpace(value)
	}

	return strings.TrimSpace(field)
}

/*
MessageID returns the original message's Message-ID without angle
brackets, or empty when unknown.
*/
func (r *Report) MessageID() string {
	if r.Original == nil {
		return ""
	}

	return strings.TrimSpace(strings.Trim(strings.TrimSpace(r.Original.Get("Message-ID")), "<>"))
}
