/*
|--------------------------------------------------------------------------
| Access Policy Requests
|--------------------------------------------------------------------------
|
| The input of PATCH /workspace/access-policy. Every key is optional and a
| list replaces the whole stored list. A session limit may be sent as null
| to clear it, which a missing key does not do, so the raw body says which
| keys were sent. An address list is limited here; whether each entry
| parses is the action's concern, since it stores the normalized form.
|
*/

package requests

import "github.com/dimmihq/dimmi/app/models"

/*
AccessPolicyBody is what a change to the rules may send.
*/
type AccessPolicyBody struct {
	AllowedSignInMethods *[]string `json:"allowed_sign_in_methods,omitempty" validate:"omitempty,max=20"`
	GoogleAllowedDomains *[]string `json:"google_allowed_domains,omitempty"  validate:"omitempty,max=20,dive,max=253"`

	MaxAge      *int `json:"session_max_age_minutes,omitempty"      validate:"omitempty,min=60,max=43200"`
	IdleTimeout *int `json:"session_idle_timeout_minutes,omitempty" validate:"omitempty,min=15,max=20160"`

	DashboardAllowlist *[]AllowedAddressBody `json:"dashboard_ip_allowlist,omitempty" validate:"omitempty,max=100,dive"`
	APIAllowlist       *[]AllowedAddressBody `json:"api_ip_allowlist,omitempty"       validate:"omitempty,max=100,dive"`
}

/*
AllowedAddressBody is one allowlist entry.
*/
type AllowedAddressBody struct {
	CIDR  string `json:"cidr"  validate:"required,max=64"`
	Label string `json:"label" validate:"required,max=60"`
}

/*
UpdateAccessPolicyRequest is the input of PATCH /workspace/access-policy.
*/
type UpdateAccessPolicyRequest struct {
	Body    AccessPolicyBody
	RawBody []byte
}

/*
AllowedAddresses returns the entries in the form the rules store them,
nil when the key was not sent.
*/
func AllowedAddresses(list *[]AllowedAddressBody) *[]models.AllowedAddress {
	if list == nil {
		return nil
	}

	out := make([]models.AllowedAddress, 0, len(*list))
	for _, entry := range *list {
		out = append(out, models.AllowedAddress{CIDR: entry.CIDR, Label: entry.Label})
	}

	return &out
}
