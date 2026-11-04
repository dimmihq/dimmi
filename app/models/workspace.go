/*
|--------------------------------------------------------------------------
| Workspace Behavior
|--------------------------------------------------------------------------
|
| Settings of a workspace, read from and written to its settings
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
	return WithSetting(settings, "security", "require_two_factor", on)
}

/*
ShowsViewersOnList reports whether the inbox list shows who views each
conversation. It does unless the workspace turned it off.
*/
func (w Workspace) ShowsViewersOnList() bool {
	var settings struct {
		Inbox struct {
			ShowViewersOnList *bool `json:"show_viewers_on_list"`
		} `json:"inbox"`
	}

	if len(w.Settings) == 0 || json.Unmarshal(w.Settings, &settings) != nil {
		return true
	}

	return settings.Inbox.ShowViewersOnList == nil || *settings.Inbox.ShowViewersOnList
}

/*
WithSetting returns the settings document with the key of the area set
to the value, keeping every other key.
*/
func WithSetting(settings json.RawMessage, area, key string, value any) (json.RawMessage, error) {
	values := map[string]json.RawMessage{}
	if len(settings) > 0 {
		if err := json.Unmarshal(settings, &values); err != nil {
			return nil, fmt.Errorf("read workspace settings: %w", err)
		}
	}

	section := map[string]json.RawMessage{}
	if raw, ok := values[area]; ok {
		if err := json.Unmarshal(raw, &section); err != nil {
			return nil, fmt.Errorf("read workspace %s settings: %w", area, err)
		}
	}

	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("write workspace setting %s: %w", key, err)
	}

	section[key] = encoded

	raw, err := json.Marshal(section)
	if err != nil {
		return nil, fmt.Errorf("write workspace %s settings: %w", area, err)
	}

	values[area] = raw

	return json.Marshal(values)
}
