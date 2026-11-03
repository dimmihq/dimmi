/*
|--------------------------------------------------------------------------
| User Identity Requests
|--------------------------------------------------------------------------
|
| Inputs of the endpoints that manage the provider accounts linked to
| the caller.
|
*/

package requests

/*
DestroyUserIdentityRequest is the input of DELETE
/user/identities/{identity}.
*/
type DestroyUserIdentityRequest struct {
	Identity string `path:"identity"`
}
