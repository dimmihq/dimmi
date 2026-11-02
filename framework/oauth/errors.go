/*
|--------------------------------------------------------------------------
| OAuth Errors
|--------------------------------------------------------------------------
|
| OAuth clients are libraries that parse one error shape: an error code
| from a fixed list and an optional description, never the application's
| own error envelope. Every refusal the server makes is an Error, and the
| endpoints write it in that shape. An authorization refusal that comes
| after the redirect URI checked out also knows where to send the user,
| so the client learns what went wrong; before that, sending the user
| anywhere would make the server an open redirect, so the application
| shows an error page instead.
|
*/

package oauth

import (
	"errors"
	stdhttp "net/http"

	"github.com/dimmihq/dimmi/framework/http"
)

/*
The error codes the server answers with.
*/
const (
	CodeInvalidRequest          = "invalid_request"
	CodeInvalidClient           = "invalid_client"
	CodeInvalidGrant            = "invalid_grant"
	CodeUnauthorizedClient      = "unauthorized_client"
	CodeUnsupportedGrantType    = "unsupported_grant_type"
	CodeUnsupportedResponseType = "unsupported_response_type"
	CodeInvalidScope            = "invalid_scope"
	CodeAccessDenied            = "access_denied"
	CodeInvalidRedirectURI      = "invalid_redirect_uri"
	CodeInvalidClientMetadata   = "invalid_client_metadata"
	CodeTemporarilyUnavailable  = "temporarily_unavailable"
	CodeServerError             = "server_error"
)

/*
maxFormBytes bounds the body of a token or revocation request.
*/
const maxFormBytes = 16 << 10

/*
Error is a refusal as the client receives it: an error code from the
OAuth registries, a description for the developer and the HTTP status
it is sent with.
*/
type Error struct {
	Code        string
	Description string
	Status      int
	redirect    string
}

/*
Error returns the code and description, for logs.
*/
func (e *Error) Error() string {
	return "oauth: " + e.Code + ": " + e.Description
}

/*
RedirectURL returns where to send the user so the client learns of the
refusal, and false when the client may not be trusted with a redirect
and the application should show an error page.
*/
func (e *Error) RedirectURL() (string, bool) {
	return e.redirect, e.redirect != ""
}

/*
refuse returns a 400 refusal.
*/
func refuse(code, description string) *Error {
	return &Error{Code: code, Description: description, Status: http.StatusBadRequest}
}

/*
errInvalidClient is the refusal for a client that is unknown or failed
to authenticate.
*/
func errInvalidClient() *Error {
	return &Error{
		Code: CodeInvalidClient, Description: "Client authentication failed.",
		Status: http.StatusUnauthorized,
	}
}

/*
errorBody is an error in the shape OAuth clients parse.
*/
type errorBody struct {
	Error       string `json:"error"`
	Description string `json:"error_description,omitempty"`
}

/*
writeError writes the error in the OAuth shape. An error that is not a
refusal is a failure of the server: it is logged and reported through
the errors renderer and reaches the client as a bare server_error.
*/
func (s *Server) writeError(w http.ResponseWriter, r *http.Request, err error) {
	refusal, ok := errors.AsType[*Error](err)
	if !ok {
		res := s.opts.Errors.Resolve(r.Context(), err)
		refusal = &Error{Code: CodeServerError, Status: res.Status}
	}

	if refusal.Code == CodeInvalidClient && r.Header.Get("Authorization") != "" {
		w.Header().Set("WWW-Authenticate", `Basic realm="oauth"`)
	}

	noStore(w)
	http.WriteJSON(w, refusal.Status, errorBody{Error: refusal.Code, Description: refusal.Description})
}

/*
noStore keeps responses carrying secrets out of every cache.
*/
func noStore(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
}

/*
parseForm reads the form body of a token or revocation request, up to
maxFormBytes.
*/
func parseForm(w http.ResponseWriter, r *http.Request) error {
	r.Body = stdhttp.MaxBytesReader(w, r.Body, maxFormBytes)
	if err := r.ParseForm(); err != nil {
		return refuse(CodeInvalidRequest, "The request body must be a form of at most 16 KB.")
	}

	return nil
}
