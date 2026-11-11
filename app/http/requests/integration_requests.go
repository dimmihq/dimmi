/*
|--------------------------------------------------------------------------
| Integration Requests
|--------------------------------------------------------------------------
|
| The inputs of the integration endpoints. Ids are public ones, and the
| page to come back to after a provider is a path on this site.
|
*/

package requests

/*
IntegrationProvidersRequest is the input of GET /integration-providers.
*/
type IntegrationProvidersRequest struct{}

/*
IntegrationsRequest is the input of GET /integrations, optionally of one
provider.
*/
type IntegrationsRequest struct {
	Provider string `query:"filter[provider]"`
}

/*
IntegrationRequest names an installation in the path.
*/
type IntegrationRequest struct {
	Integration string `path:"integration"`
}

/*
IntegrationInstallationBody says which features an install is for,
every feature when empty, the page to come back to, and for a service
hosted per customer, such as Zendesk, the account to connect.
*/
type IntegrationInstallationBody struct {
	Features []string `json:"features,omitempty"  validate:"omitempty,max=20,dive,min=1,max=40"`
	ReturnTo string   `json:"return_to,omitempty" validate:"omitempty,max=2000"`
	Account  string   `json:"account,omitempty"   validate:"omitempty,max=63"`
}

/*
StoreIntegrationInstallationRequest is the input of POST
/integrations/{provider}/installations.
*/
type StoreIntegrationInstallationRequest struct {
	Provider string `path:"provider"`
	Body     IntegrationInstallationBody
}
