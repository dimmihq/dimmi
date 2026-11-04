/*
|--------------------------------------------------------------------------
| Widget Update Controller
|--------------------------------------------------------------------------
|
| The polling fallback of a messenger whose stream is blocked. Each answer
| carries a signed cursor for the next poll and how long to wait before it;
| a page that left changes behind asks to poll again at once.
|
*/

package widget

import (
	"context"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	actions "github.com/dimmihq/dimmi/app/actions/widget"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
UpdateItem is one change in a poll's answer.
*/
type UpdateItem struct {
	Event   string `json:"event"`
	ID      string `json:"id"`
	Payload any    `json:"payload"`
}

/*
UpdatesOutput is a poll's answer.
*/
type UpdatesOutput struct {
	Body struct {
		Data []UpdateItem `json:"data"`
		Meta struct {
			Cursor      string    `json:"cursor"`
			PollAfterMS int64     `json:"poll_after_ms"`
			HasMore     bool      `json:"has_more"`
			ServerTime  time.Time `json:"server_time"`
		} `json:"meta"`
	}
}

/*
UpdatesRequest is the input of GET /widget/v1/updates.
*/
type UpdatesRequest struct {
	Since string `query:"since"`
}

/*
UpdateController answers polling messengers. Interval is how long a
messenger waits between polls.
*/
type UpdateController struct {
	List     *actions.ListWidgetUpdates
	Cursors  *pagination.Cursors
	Clock    clock.Clock
	Interval time.Duration
}

/*
Index returns what changed since the cursor.
*/
func (c *UpdateController) Index(ctx context.Context, in *UpdatesRequest) (*UpdatesOutput, error) {
	var since *actions.Cursor

	if in.Since != "" {
		var cursor actions.Cursor
		if err := c.Cursors.Decode(in.Since, &cursor); err != nil {
			return nil, pagination.ErrInvalidCursor
		}

		since = &cursor
	}

	page, err := c.List.Handle(ctx, authenticated(ctx), since)
	if err != nil {
		return nil, err
	}

	out := &UpdatesOutput{}
	out.Body.Data = make([]UpdateItem, 0, len(page.Items))

	for _, item := range page.Items {
		out.Body.Data = append(out.Body.Data, updateItem(item))
	}

	if out.Body.Meta.Cursor, err = c.Cursors.Encode(page.Next); err != nil {
		return nil, err
	}

	out.Body.Meta.HasMore, out.Body.Meta.ServerTime = page.HasMore, c.Clock.Now().UTC()
	if !page.HasMore {
		out.Body.Meta.PollAfterMS = c.Interval.Milliseconds()
	}

	return out, nil
}

/*
updateItem shapes one change the way the stream would have carried it.
*/
func updateItem(u actions.Update) UpdateItem {
	item := UpdateItem{Event: u.Event, ID: ids.Encode("evt", ids.New())}

	if u.Message != nil {
		conv := models.PublicID(models.PrefixConversation, u.Message.Message.ConversationID)
		item.Payload = map[string]any{"conversation_id": conv, "message": resources.NewWidgetMessage(*u.Message)}

		return item
	}

	widget := resources.NewWidgetConversation(actions.VisibleConversation{Conversation: *u.Conversation})
	item.Payload = map[string]any{"conversation_id": widget.ID, "widget_id": widget.WidgetID, "conversation": widget}

	return item
}
