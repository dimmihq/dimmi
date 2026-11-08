/*
|--------------------------------------------------------------------------
| Widget Help Pass Requests
|--------------------------------------------------------------------------
|
| The messenger asks where to open an article of its help center, so a
| visitor it knows arrives there signed in.
|
*/

package requests

/*
StoreWidgetHelpPassRequest is the input of POST /widget/v1/help-passes:
the slug of the article to open, empty for the help center's home.
*/
type StoreWidgetHelpPassRequest struct {
	Body struct {
		Article string `json:"article,omitempty" maxLength:"200" pattern:"^[a-z0-9-]*$"`
	}
}
