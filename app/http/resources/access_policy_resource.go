/*
|--------------------------------------------------------------------------
| Access Policy Resource
|--------------------------------------------------------------------------
|
| A workspace's rules as the settings page edits them. The mirrors of the
| organization's single sign-on connections stay inside: they only exist
| so a rule can read them without a query.
|
*/

package resources

import (
	"github.com/dimmihq/dimmi/app/actions/accesspolicy"
	"github.com/dimmihq/dimmi/app/models"
)

/*
AccessPolicy is a workspace's rules. SSOEnforced says whether a
connection of the organization is enforced. AvailableSignInMethods lists
the ways of signing in this install offers, YourIP the address the
caller is seen at and TokensOutside the live tokens the API allowlist
would stop.
*/
type AccessPolicy struct {
	Object                 string                  `json:"object"`
	AllowedSignInMethods   []string                `json:"allowed_sign_in_methods"`
	GoogleAllowedDomains   []string                `json:"google_allowed_domains"`
	SessionMaxAge          *int                    `json:"session_max_age_minutes"`
	SessionIdleTimeout     *int                    `json:"session_idle_timeout_minutes"`
	DashboardAllowlist     []models.AllowedAddress `json:"dashboard_ip_allowlist"`
	APIAllowlist           []models.AllowedAddress `json:"api_ip_allowlist"`
	RequireTwoFactor       bool                    `json:"require_two_factor"`
	SSOEnforced            bool                    `json:"sso_enforced"`
	AvailableSignInMethods []string                `json:"available_sign_in_methods"`
	YourIP                 string                  `json:"your_ip"`
	TokensOutside          int                     `json:"tokens_outside"`
}

/*
NewAccessPolicy shapes the rules.
*/
func NewAccessPolicy(p accesspolicy.Policy) AccessPolicy {
	s := p.Settings

	yourIP := ""
	if p.YourIP.IsValid() {
		yourIP = p.YourIP.String()
	}

	return AccessPolicy{
		Object:                 "access_policy",
		AllowedSignInMethods:   orEmpty(s.AllowedSignInMethods),
		GoogleAllowedDomains:   orEmpty(s.GoogleAllowedDomains),
		SessionMaxAge:          s.SessionMaxAge,
		SessionIdleTimeout:     s.SessionIdleTimeout,
		DashboardAllowlist:     orEmpty(s.DashboardAllowlist),
		APIAllowlist:           orEmpty(s.APIAllowlist),
		RequireTwoFactor:       s.RequireTwoFactor,
		SSOEnforced:            len(s.SSOEnforcedConnectionIDs) > 0,
		AvailableSignInMethods: p.Available,
		YourIP:                 yourIP,
		TokensOutside:          p.TokensOutside,
	}
}

/*
orEmpty returns an empty list for nil, so a client never reads null where
it expects a list.
*/
func orEmpty[T any](list []T) []T {
	if list == nil {
		return []T{}
	}

	return list
}
