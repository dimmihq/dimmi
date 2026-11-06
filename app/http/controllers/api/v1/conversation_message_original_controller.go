/*
|--------------------------------------------------------------------------
| Original Messages And Delivery
|--------------------------------------------------------------------------
|
| GET .../messages/{message}/original reads the whole of a received
| email, and .../original/raw downloads it as it arrived. That file is
| written by a stranger, so it is only ever an attachment, never sniffed,
| never cached, and sandboxed should a browser open it all the same.
| POST .../messages/{message}/retry sends a reply that was not delivered
| again and answers 200 with it, pending once more.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/actions/messages"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
OriginalOutput is the whole of a received message.
*/
type OriginalOutput struct {
	Body fhttp.Resource[resources.MessageOriginal]
}

/*
RawOutput is a received email as it arrived, as a file to save.
*/
type RawOutput struct {
	ContentType        string `header:"Content-Type"`
	ContentDisposition string `header:"Content-Disposition"`
	ContentPolicy      string `header:"Content-Security-Policy"`
	Sniffing           string `header:"X-Content-Type-Options"`
	Cache              string `header:"Cache-Control"`
	Body               []byte
}

/*
ShowOriginal reads the whole of a received message.
*/
func (c *ConversationMessageController) ShowOriginal(
	ctx context.Context, in *requests.ConversationMessageRequest,
) (*OriginalOutput, error) {
	input, err := c.originalInput(ctx, in)
	if err != nil {
		return nil, err
	}

	original, err := c.Original.Handle(ctx, input)
	if err != nil {
		return nil, err
	}

	return &OriginalOutput{Body: fhttp.NewResource(resources.NewMessageOriginal(original))}, nil
}

/*
Retry sends a reply that was not delivered again.
*/
func (c *ConversationMessageController) Retry(
	ctx context.Context, in *requests.ConversationMessageRequest,
) (*WrittenOutput, error) {
	input, err := c.originalInput(ctx, in)
	if err != nil {
		return nil, err
	}

	written, err := c.Resend.Handle(ctx, messages.RetryInput{
		Membership: input.Membership, Ref: input.Ref, MessageID: input.MessageID, MutationID: mutationID(ctx),
	})
	if err != nil {
		return nil, err
	}

	out := writtenOutput(written)
	out.Status = fhttp.StatusOK

	return out, nil
}

/*
DownloadRaw answers with a received email as it arrived, under headers
that keep a browser from ever running it.
*/
func (c *ConversationMessageController) DownloadRaw(
	ctx context.Context, in *requests.ConversationMessageRequest,
) (*RawOutput, error) {
	input, err := c.originalInput(ctx, in)
	if err != nil {
		return nil, err
	}

	raw, err := c.Raw.Handle(ctx, input)
	if err != nil {
		return nil, err
	}

	return &RawOutput{
		ContentType:        "application/octet-stream",
		ContentDisposition: `attachment; filename="` + raw.Filename + `"`,
		ContentPolicy:      "default-src 'none'; sandbox",
		Sniffing:           "nosniff",
		Cache:              "no-store",
		Body:               raw.Content,
	}, nil
}

/*
originalInput reads which message of which conversation a request names.
*/
func (c *ConversationMessageController) originalInput(
	ctx context.Context, in *requests.ConversationMessageRequest,
) (messages.OriginalInput, error) {
	ref, err := conversationRef(c.IDs, in.Conversation)
	if err != nil {
		return messages.OriginalInput{}, err
	}

	id, err := c.IDs.Decode(string(models.PrefixMessage), in.Message)
	if err != nil {
		return messages.OriginalInput{}, exceptions.ErrNotFound
	}

	return messages.OriginalInput{Membership: membership(ctx), Ref: ref, MessageID: id}, nil
}
