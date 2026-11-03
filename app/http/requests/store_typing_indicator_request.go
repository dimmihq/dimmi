/*
|--------------------------------------------------------------------------
| Store Typing Indicator Request
|--------------------------------------------------------------------------
|
| A client reports that its user started or stopped typing in a
| conversation's reply or note composer. Both fields may be left out: a
| bare report means someone started typing a reply.
|
*/

package requests

/*
StoreTypingIndicatorRequest is the input of
POST /conversations/{conversation}/typing-indicators.
*/
type StoreTypingIndicatorRequest struct {
	Conversation string `path:"conversation"`
	Body         struct {
		State    *string `json:"state,omitempty"    validate:"omitempty,oneof=start stop"`
		Composer *string `json:"composer,omitempty" validate:"omitempty,oneof=reply note"`
	}
}
