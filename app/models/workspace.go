/*
|--------------------------------------------------------------------------
| Workspace Behavior
|--------------------------------------------------------------------------
|
| Settings of a workspace, read from and written to its settings document
| in one place, so the code that follows a setting and the endpoint that
| changes it always agree on where it lives. Settings nest
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

/*
CustomerFacingName returns the name customers see in messages and mail
from the workspace: the one set for them, or else the workspace's own
name.
*/
func (w Workspace) CustomerFacingName() string {
	if name := w.CustomerFacingNameSetting(); name != nil {
		return *name
	}

	return w.Name
}

/*
CustomerFacingNameSetting returns the name set for customers, nil when
none is, so customers see the workspace's own name.
*/
func (w Workspace) CustomerFacingNameSetting() *string {
	var settings struct {
		CustomerFacingName *string `json:"customer_facing_name"`
	}

	if len(w.Settings) == 0 || json.Unmarshal(w.Settings, &settings) != nil {
		return nil
	}

	if settings.CustomerFacingName == nil || *settings.CustomerFacingName == "" {
		return nil
	}

	return settings.CustomerFacingName
}

/*
WithCustomerFacingName returns the settings document with the name
customers see set, or removed when name is nil, keeping every other key.
*/
func WithCustomerFacingName(settings json.RawMessage, name *string) (json.RawMessage, error) {
	values := map[string]json.RawMessage{}
	if len(settings) > 0 {
		if err := json.Unmarshal(settings, &values); err != nil {
			return nil, fmt.Errorf("read workspace settings: %w", err)
		}
	}

	if name == nil {
		delete(values, "customer_facing_name")

		return json.Marshal(values)
	}

	raw, err := json.Marshal(*name)
	if err != nil {
		return nil, fmt.Errorf("write customer facing name: %w", err)
	}

	values["customer_facing_name"] = raw

	return json.Marshal(values)
}
