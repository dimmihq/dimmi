/*
|--------------------------------------------------------------------------
| Widget AI Requests
|--------------------------------------------------------------------------
|
| The visitor's two AI actions: mark an answer helpful and ask for a
| person.
|
*/

package requests

/*
StoreWidgetAiHelpRequest is the input of POST .../ai/helped.
*/
type StoreWidgetAiHelpRequest struct {
	Conversation string `path:"conversation"`
}

/*
StoreWidgetAiHandoffRequest is the input of POST .../ai/handoff.
*/
type StoreWidgetAiHandoffRequest struct {
	Conversation string `path:"conversation"`
}
