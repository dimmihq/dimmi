/*
|--------------------------------------------------------------------------
| Widget Requests
|--------------------------------------------------------------------------
|
| Inputs of the routes a visitor's browser calls.
|
*/

package requests

/*
ShowWidgetConfigRequest is the input of GET /apps/{app}/config. Version only
keeps a CDN from answering with an older copy.
*/
type ShowWidgetConfigRequest struct {
	App         string `path:"app"`
	Locale      string `query:"locale"`
	Version     string `query:"v"`
	IfNoneMatch string `header:"If-None-Match"`
}

/*
ShowWidgetAvailabilityRequest is the input of GET /apps/{app}/availability.
*/
type ShowWidgetAvailabilityRequest struct {
	App    string `path:"app"`
	Locale string `query:"locale"`
}
