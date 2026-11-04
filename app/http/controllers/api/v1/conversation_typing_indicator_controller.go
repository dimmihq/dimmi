/*
|--------------------------------------------------------------------------
| Conversation Typing Indicator Controller
|--------------------------------------------------------------------------
|
| POST /conversations/{conversation}/typing-indicators is how every
| client says its user is typing: the dashboard, apps, the API and
| agents alike. The report is accepted, never stored, and answered 202
| whether or not anyone is told. A conversation the caller cannot see
| is not found.
|
*/

package v1

import (
	"cmp"
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
TypingIndicatorOutput is the accepted report.
*/
type TypingIndicatorOutput struct {
	Body fhttp.Resource[resources.TypingIndicator]
}

/*
ConversationTypingIndicatorController accepts typing reports.
*/
type ConversationTypingIndicatorController struct {
	RecordTyping *conversations.RecordTyping
	IDs          *ids.Registry
}

/*
Store accepts a typing report.
*/
func (c *ConversationTypingIndicatorController) Store(
	ctx context.Context, in *requests.StoreTypingIndicatorRequest,
) (*TypingIndicatorOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixConversation), in.Conversation)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	indicator, err := c.RecordTyping.Handle(ctx, conversations.RecordTypingInput{
		Membership:     membership(ctx),
		ConversationID: id,
		State:          cmp.Or(typingField(in.Body.State), conversations.TypingStart),
		Composer:       cmp.Or(typingField(in.Body.Composer), conversations.ComposerReply),
	})
	if err != nil {
		return nil, err
	}

	return &TypingIndicatorOutput{Body: fhttp.NewResource(resources.NewTypingIndicator(indicator))}, nil
}

/*
typingField returns the field, or "" when there is none.
*/
func typingField(s *string) string {
	if s == nil {
		return ""
	}

	return *s
}
