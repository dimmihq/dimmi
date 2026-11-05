/*
|--------------------------------------------------------------------------
| Link Safety Settings
|--------------------------------------------------------------------------
|
| How the inbox treats links in what customers send. The checks run in
| the teammate's browser when a message is shown, never on the stored
| text, so a change takes effect at once on every message. A workspace
| that never chose has the checks on, no allowed or blocked domains, and
| email links shown as links.
|
*/

package models

import (
	"encoding/json"
	"fmt"
)

/*
LinkSafety is a workspace's link checks. Enabled turns the warnings on;
AllowDomains skip them and BlockDomains make a link unclickable, each
matching the domain and every name under it. StripInboundEmailLinks
shows the links of email as plain text until a teammate reveals them.
*/
type LinkSafety struct {
	Enabled                bool     `json:"enabled"`
	AllowDomains           []string `json:"allow_domains"`
	BlockDomains           []string `json:"block_domains"`
	StripInboundEmailLinks bool     `json:"strip_inbound_email_links"`
}

/*
LinkSafety returns the workspace's link checks over the defaults.
*/
func (w Workspace) LinkSafety() LinkSafety {
	var settings struct {
		LinkSafety LinkSafety `json:"link_safety"`
	}

	settings.LinkSafety = LinkSafety{Enabled: true, AllowDomains: []string{}, BlockDomains: []string{}}

	if len(w.Settings) > 0 && json.Unmarshal(w.Settings, &settings) != nil {
		return LinkSafety{Enabled: true, AllowDomains: []string{}, BlockDomains: []string{}}
	}

	if settings.LinkSafety.AllowDomains == nil {
		settings.LinkSafety.AllowDomains = []string{}
	}

	if settings.LinkSafety.BlockDomains == nil {
		settings.LinkSafety.BlockDomains = []string{}
	}

	return settings.LinkSafety
}

/*
WithArea returns the settings document with the whole area replaced by
the value, keeping every other area.
*/
func WithArea(settings json.RawMessage, area string, value any) (json.RawMessage, error) {
	values := map[string]any{}
	if len(settings) > 0 {
		if err := json.Unmarshal(settings, &values); err != nil {
			return nil, fmt.Errorf("read workspace settings: %w", err)
		}
	}

	values[area] = value

	return json.Marshal(values)
}
