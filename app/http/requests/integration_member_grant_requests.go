/*
|--------------------------------------------------------------------------
| Integration Member Grant Requests
|--------------------------------------------------------------------------
|
| The inputs of a teammate's own grant on an installation: the page to
| come back to, and the grant to take back.
|
*/

package requests

/*
IntegrationMemberGrantBody is the page to come back to after a
teammate's grant.
*/
type IntegrationMemberGrantBody struct {
	ReturnTo string `json:"return_to,omitempty" validate:"omitempty,max=2000"`
}

/*
StoreIntegrationMemberGrantRequest is the input of POST
/integrations/{integration}/member-grants.
*/
type StoreIntegrationMemberGrantRequest struct {
	Integration string `path:"integration"`
	Body        IntegrationMemberGrantBody
}

/*
DestroyIntegrationMemberGrantRequest is the input of DELETE
/integrations/{integration}/member-grants/{grant}.
*/
type DestroyIntegrationMemberGrantRequest struct {
	Integration string `path:"integration"`
	Grant       string `path:"grant"`
}
