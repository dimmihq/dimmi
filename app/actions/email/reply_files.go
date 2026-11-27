/*
|--------------------------------------------------------------------------
| Reply Files
|--------------------------------------------------------------------------
|
| The files a teammate attached to a reply travel with the email. They are
| read from storage when the mail is put together, so a retry sends the
| same files again, and a file that cannot be read fails the send rather
| than leaving silently without it. Mail providers refuse a message past
| about 25 MB, so files go along while they fit and the rest are linked
| instead: the reply still sends, with a signed link to download each
| file that stayed behind.
|
*/

package email

import (
	"context"
	"fmt"
	"html"
	"io"
	"strings"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/mail"

	"github.com/dimmihq/dimmi/app/contracts"
)

/*
MaxAttachedBytes is how much file a reply carries attached, before
encoding: base64 makes it about a third larger, so with the message
around it the mail stays under the 25 MB providers accept.
*/
const MaxAttachedBytes = 18 << 20

/*
FileLinks signs links that download a reply's file without signing in.
*/
type FileLinks interface {
	// FileLink returns the link that downloads the file of the workspace.
	FileLink(workspaceID, attachmentID ids.UUID) (string, error)
}

/*
linkedFile is a file that stayed behind with the link that downloads it.
*/
type linkedFile struct {
	file contracts.OutboundAttachment
	url  string
}

/*
attachFiles reads the files that fit into the mail and links the rest
in its text, falling back to attaching everything when there is no way
to sign links.
*/
func (a *SendEmailMessage) attachFiles(ctx context.Context, out *outbound) error {
	attached, behind := fitFiles(out.msg.Attachments, MaxAttachedBytes)
	if a.Links == nil {
		attached, behind = out.msg.Attachments, nil
	}

	files, err := readFiles(ctx, attached)
	if err != nil {
		return err
	}

	out.draft.Attachments = files

	links := make([]linkedFile, 0, len(behind))
	for _, f := range behind {
		url, err := a.Links.FileLink(out.msg.WorkspaceID, f.ID)
		if err != nil {
			return fmt.Errorf("link reply file %s: %w", f.Filename, err)
		}

		links = append(links, linkedFile{file: f, url: url})
	}

	addLinks(&out.draft.Text, &out.draft.HTML, links)

	return nil
}

/*
fitFiles splits the files into those that go along, in the order
attached while they fit within the budget, pictures shown in the body
first, and those that stay behind.
*/
func fitFiles(files []contracts.OutboundAttachment, budget int64) (attached, behind []contracts.OutboundAttachment) {
	var used int64

	for _, inline := range []bool{true, false} {
		for _, f := range files {
			if f.Inline != inline || f.Open == nil {
				continue
			}

			if used+f.SizeBytes <= budget {
				used += f.SizeBytes
				attached = append(attached, f)

				continue
			}

			behind = append(behind, f)
		}
	}

	return attached, behind
}

/*
addLinks adds the links of the files that stayed behind below the
reply's text and HTML.
*/
func addLinks(text, body *string, links []linkedFile) {
	if len(links) == 0 {
		return
	}

	const intro = "Some files were too large to attach. Download them here:"

	var t, h strings.Builder

	t.WriteString("\n\n" + intro + "\n")
	h.WriteString("<p>" + intro + "</p><ul>")

	for _, l := range links {
		size := sizeLabel(l.file.SizeBytes)
		fmt.Fprintf(&t, "- %s (%s): %s\n", l.file.Filename, size, l.url)
		fmt.Fprintf(&h, `<li><a href="%s">%s</a> (%s)</li>`, html.EscapeString(l.url),
			html.EscapeString(l.file.Filename), size)
	}

	h.WriteString("</ul>")

	*text += t.String()
	if *body != "" {
		*body += h.String()
	}
}

/*
sizeLabel writes a file size in megabytes, or kilobytes below one.
*/
func sizeLabel(n int64) string {
	if n < 1<<20 {
		return fmt.Sprintf("%d KB", max(n>>10, 1))
	}

	return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
}

/*
readFiles reads each attached file into the mail.
*/
func readFiles(ctx context.Context, files []contracts.OutboundAttachment) ([]mail.Attachment, error) {
	out := make([]mail.Attachment, 0, len(files))
	for _, file := range files {
		if file.Open == nil {
			continue
		}

		body, err := readFile(ctx, file)
		if err != nil {
			return nil, err
		}

		out = append(out, mail.Attachment{
			Filename: file.Filename, ContentType: file.ContentType, Content: body,
			Inline: file.Inline, ContentID: file.ContentID,
		})
	}

	return out, nil
}

func readFile(ctx context.Context, file contracts.OutboundAttachment) ([]byte, error) {
	r, err := file.Open(ctx)
	if err != nil {
		return nil, fmt.Errorf("open reply file %s: %w", file.Filename, err)
	}

	defer func() { _ = r.Close() }()

	body, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read reply file %s: %w", file.Filename, err)
	}

	return body, nil
}
