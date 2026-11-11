/*
|--------------------------------------------------------------------------
| Integration Provider
|--------------------------------------------------------------------------
|
| A provider names the tool an installation connects: Slack today, other
| tools later. The set grows one provider at a time, so the database
| only checks a value's shape and each provider names itself here.
|
*/

package enums

/*
IntegrationProvider names the tool an installation connects.
*/
type IntegrationProvider string

/*
The providers the product ships.
*/
const (
	IntegrationProviderSlack    IntegrationProvider = "slack"
	IntegrationProviderNotion   IntegrationProvider = "notion"
	IntegrationProviderIntercom IntegrationProvider = "intercom"
	IntegrationProviderZendesk  IntegrationProvider = "zendesk"
	IntegrationProviderGitHub   IntegrationProvider = "github"
)

/*
IntegrationProviders returns every provider, in display order.
*/
func IntegrationProviders() []IntegrationProvider {
	return []IntegrationProvider{
		IntegrationProviderSlack, IntegrationProviderNotion, IntegrationProviderIntercom, IntegrationProviderZendesk,
		IntegrationProviderGitHub,
	}
}
