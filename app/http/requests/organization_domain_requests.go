/*
|--------------------------------------------------------------------------
| Organization Domain Requests
|--------------------------------------------------------------------------
|
| Inputs of the endpoints on the domains a company proves it owns. A
| domain is named by its public id, and a new one is sent as typed; the
| action works out its canonical form.
|
*/

package requests

/*
DomainRequest is the input of the endpoints on one domain.
*/
type DomainRequest struct {
	Domain string `path:"domain"`
}

/*
StoreDomainRequest is the input of POST /organization-domains.
*/
type StoreDomainRequest struct {
	Body struct {
		Domain string `json:"domain" validate:"required,max=253"`
	}
}

/*
UpdateDomainRequest is the input of PATCH /organization-domains/{domain}:
the connection its sign-ins go to, null for none.
*/
type UpdateDomainRequest struct {
	Domain string `path:"domain"`
	Body   struct {
		SSOConnectionID *string `json:"sso_connection_id"`
	}
}
