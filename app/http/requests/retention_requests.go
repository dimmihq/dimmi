/*
|--------------------------------------------------------------------------
| Retention Requests
|--------------------------------------------------------------------------
|
| Input of the retention preview: the period, in days, to count against.
| Its range is checked by the action.
|
*/

package requests

/*
RetentionPreviewRequest is the input of POST /retention-previews.
*/
type RetentionPreviewRequest struct {
	Body struct {
		ClosedConversationDays int `json:"closed_conversation_days" validate:"required"`
	}
}
