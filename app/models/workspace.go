/*
|--------------------------------------------------------------------------
| Workspace Behavior
|--------------------------------------------------------------------------
|
| Security settings of a workspace, read from and written to its settings
| document in one place, so the policy that enforces a setting and the
| endpoint that changes it always agree on where it lives. Settings nest
| by area, such as {"security": {"require_two_factor": true}}, and every
| key a change does not touch is kept as it was.
|
*/

package models

import (
	"encoding/json"
	"fmt"
)

/*
securitySettings is the security area of a workspace's settings.
*/
type securitySettings struct {
	RequireTwoFactor bool `json:"require_two_factor"`
}

/*
RequiresTwoFactor reports whether every member must protect their
account with a second factor to work in the workspace. Settings that do
not read as a document require nothing.
*/
func (w Workspace) RequiresTwoFactor() bool {
	var settings struct {
		Security securitySettings `json:"security"`
	}

	if len(w.Settings) == 0 || json.Unmarshal(w.Settings, &settings) != nil {
		return false
	}

	return settings.Security.RequireTwoFactor
}

/*
WithTwoFactorRequired returns the settings document with the
requirement of a second factor set to on, keeping every other key.
*/
func WithTwoFactorRequired(settings json.RawMessage, on bool) (json.RawMessage, error) {
	values := map[string]json.RawMessage{}
	if len(settings) > 0 {
		if err := json.Unmarshal(settings, &values); err != nil {
			return nil, fmt.Errorf("read workspace settings: %w", err)
		}
	}

	security := map[string]json.RawMessage{}
	if raw, ok := values["security"]; ok {
		if err := json.Unmarshal(raw, &security); err != nil {
			return nil, fmt.Errorf("read workspace security settings: %w", err)
		}
	}

	security["require_two_factor"] = json.RawMessage(fmt.Sprint(on))

	raw, err := json.Marshal(security)
	if err != nil {
		return nil, fmt.Errorf("write workspace security settings: %w", err)
	}

	values["security"] = raw

	return json.Marshal(values)
}
