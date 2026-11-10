/*
|--------------------------------------------------------------------------
| Slack Files
|--------------------------------------------------------------------------
|
| A reply's files go to Slack the way Slack now asks: an upload address
| per file, the bytes posted there, then one call that shares them all
| into the reply's thread. Files go after the text, so the customer
| reads the reply first.
|
*/

package slack

import (
	"context"
	"fmt"

	"github.com/slack-go/slack"

	"github.com/dimmihq/dimmi/app/contracts"
)

/*
upload shares the files into the thread and returns their Slack ids.
*/
func (a *API) upload(ctx context.Context, to target, files []contracts.OutboundAttachment) ([]string, error) {
	if len(files) == 0 {
		return nil, nil
	}

	summaries := make([]slack.FileSummary, 0, len(files))

	for _, file := range files {
		id, err := a.uploadOne(ctx, file)
		if err != nil {
			return nil, err
		}

		summaries = append(summaries, slack.FileSummary{ID: id, Title: file.Filename})
	}

	_, err := a.client.CompleteUploadExternalContext(ctx, slack.CompleteUploadExternalParameters{
		Files: summaries, Channel: to.channel, ThreadTimestamp: to.threadTS,
	})
	if err != nil {
		return nil, MapError("files.completeUploadExternal", err)
	}

	out := make([]string, 0, len(summaries))
	for _, s := range summaries {
		out = append(out, s.ID)
	}

	return out, nil
}

/*
uploadOne posts one file's bytes to its upload address.
*/
func (a *API) uploadOne(ctx context.Context, file contracts.OutboundAttachment) (string, error) {
	reserved, err := a.client.GetUploadURLExternalContext(ctx, slack.GetUploadURLExternalParameters{
		FileName: file.Filename, FileSize: int(file.SizeBytes),
	})
	if err != nil {
		return "", MapError("files.getUploadURLExternal", err)
	}

	content, err := file.Open(ctx)
	if err != nil {
		return "", fmt.Errorf("open reply file: %w", err)
	}
	defer func() { _ = content.Close() }()

	err = a.client.UploadToURL(ctx, slack.UploadToURLParameters{
		UploadURL: reserved.UploadURL, Reader: content, Filename: file.Filename,
	})
	if err != nil {
		return "", MapError("upload file", err)
	}

	return reserved.FileID, nil
}
