/*
|--------------------------------------------------------------------------
| Connected App Requests
|--------------------------------------------------------------------------
|
| The inputs of the consent screen and the list of connected apps. The
| app's request travels as its original query, so the server checks it
| again instead of trusting what the browser kept of it.
|
*/

package requests

/*
OAuthAuthorizationRequest is the input of GET /oauth/authorization: the
query the app sent to the authorization page.
*/
type OAuthAuthorizationRequest struct {
	Query string `query:"query" validate:"required,max=8000"`
}

/*
DecideOAuthAuthorizationRequest is the input of POST
/oauth/authorizations: the app's query, the person's decision, and for
an approval the workspace and the access they give.
*/
type DecideOAuthAuthorizationRequest struct {
	Body struct {
		Query       string   `json:"query"                  validate:"required,max=8000"`
		Decision    string   `json:"decision"               validate:"required,oneof=approve deny"`
		WorkspaceID string   `json:"workspace_id,omitempty" validate:"omitempty,max=64"`
		Scopes      []string `json:"scopes,omitempty"       validate:"omitempty,max=60,dive,max=60"`
	}
}

/*
OAuthAppRequest names one connected app.
*/
type OAuthAppRequest struct {
	App string `path:"app"`
}
