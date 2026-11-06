/*
|--------------------------------------------------------------------------
| Widget Preview Requests
|--------------------------------------------------------------------------
|
| Input of the endpoint that shows a widget's config with unsaved changes:
| the same body a save sends, and the language to show it in.
|
*/

package requests

/*
PreviewWidgetSettingsRequest is the input of POST
/widget-settings/{widget}/preview.
*/
type PreviewWidgetSettingsRequest struct {
	Widget  string `path:"widget"`
	Locale  string `query:"locale" validate:"omitempty,max=35"`
	Body    WidgetSettingsBody
	RawBody []byte
}
