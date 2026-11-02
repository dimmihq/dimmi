/*
|--------------------------------------------------------------------------
| Session Requests
|--------------------------------------------------------------------------
|
| Requests that point at the caller's own sessions. A session is named by
| its public id, or by "current" for the one the request came from.
|
*/

package requests

/*
CurrentSession is how a request names the session it came from.
*/
const CurrentSession = "current"

/*
DestroySessionRequest is the input of DELETE /sessions/{session}.
*/
type DestroySessionRequest struct {
	Session string `path:"session"`
}

/*
NoInput is the input of an endpoint that reads nothing from the request
beyond its credential.
*/
type NoInput struct{}

/*
WorkspaceRequest is the input of an endpoint that may work inside the
workspace a browser or app names in the Dimmi-Workspace header.
*/
type WorkspaceRequest struct {
	Workspace string `header:"Dimmi-Workspace"`
}
