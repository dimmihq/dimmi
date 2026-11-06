/*
|--------------------------------------------------------------------------
| Reply Files
|--------------------------------------------------------------------------
|
| The files a teammate attached to a reply travel with the email. They are
| read from storage when the mail is put together, so a retry sends the
| same files again, and a file that cannot be read fails the send rather
| than leaving silently without it.
|
*/

package email

import (
	"context"
	"fmt"
	"io"

	"github.com/dimmihq/dimmi/framework/mail"

	"github.com/dimmihq/dimmi/app/contracts"
)

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
