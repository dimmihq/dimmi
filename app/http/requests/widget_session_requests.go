/*
|--------------------------------------------------------------------------
| Widget Session Requests
|--------------------------------------------------------------------------
|
| What a visitor's messenger sends to start a session. Browsers of every
| age send it, so keys it does not know are ignored rather than refused,
| and sizes are bounded before anything reaches an action.
|
*/

package requests

import "github.com/dimmihq/dimmi/app/actions/widget"

/*
WidgetPageBody is one page a visitor was on.
*/
type WidgetPageBody struct {
	_        struct{} `additionalProperties:"true"`
	URL      string   `json:"url"                maxLength:"4096"`
	Title    string   `json:"title,omitempty"    maxLength:"1000"`
	Referrer string   `json:"referrer,omitempty" maxLength:"4096"`
}

/*
Page returns the page as the actions take it.
*/
func (p WidgetPageBody) Page() widget.Page {
	return widget.Page{URL: p.URL, Title: p.Title, Referrer: p.Referrer}
}

/*
WidgetSDKBody names the client library and its version.
*/
type WidgetSDKBody struct {
	_       struct{} `additionalProperties:"true"`
	Name    string   `json:"name,omitempty"    maxLength:"64"`
	Version string   `json:"version,omitempty" maxLength:"64"`
}

/*
StoreWidgetSessionBody is the body of POST /widget/v1/sessions.
*/
type StoreWidgetSessionBody struct {
	_             struct{}         `additionalProperties:"true"`
	AppID         string           `json:"app_id"                   maxLength:"64"`
	UserJWT       *string          `json:"user_jwt,omitempty"       maxLength:"8192"`
	PreviousToken *string          `json:"previous_token,omitempty" maxLength:"128"`
	TestToken     *string          `json:"test_token,omitempty"     maxLength:"128"`
	Page          *WidgetPageBody  `json:"page,omitempty"`
	Pages         []WidgetPageBody `json:"pages,omitempty"          maxItems:"10"`
	Locale        string           `json:"locale,omitempty"         maxLength:"64"`
	Timezone      string           `json:"timezone,omitempty"       maxLength:"64"`
	Context       map[string]any   `json:"context,omitempty"`
	Website       string           `json:"website,omitempty"        maxLength:"1000"`
	SDK           *WidgetSDKBody   `json:"sdk,omitempty"`
}

/*
StoreWidgetSessionRequest is the input of POST /widget/v1/sessions.
*/
type StoreWidgetSessionRequest struct {
	Body StoreWidgetSessionBody
}

/*
WidgetPages returns the pages the browser buffered before the session.
*/
func (b StoreWidgetSessionBody) WidgetPages() []widget.Page {
	out := make([]widget.Page, 0, len(b.Pages))
	for _, p := range b.Pages {
		out = append(out, p.Page())
	}

	return out
}
