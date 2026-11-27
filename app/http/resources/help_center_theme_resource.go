/*
|--------------------------------------------------------------------------
| Help Center Theme Resources
|--------------------------------------------------------------------------
|
| How a version of a help center's theme reads over the API: what its
| theme.json says, whether the help center wears it, its size, and the
| settings it declares with the team's values for them.
|
*/

package resources

import (
	"encoding/json"
	"time"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/resources/views/themes"
)

/*
HelpCenterThemeSetting is a setting a theme declares.
*/
type HelpCenterThemeSetting struct {
	ID      string   `json:"id"`
	Type    string   `json:"type"`
	Label   string   `json:"label"`
	Info    string   `json:"info,omitempty"`
	Default any      `json:"default,omitempty"`
	Options []string `json:"options,omitempty"`
}

/*
HelpCenterTheme is one version of a help center's theme.
*/
type HelpCenterTheme struct {
	ID           string                   `json:"id"`
	Object       string                   `json:"object"`
	HelpCenterID string                   `json:"help_center_id"`
	Version      int32                    `json:"version"`
	Name         string                   `json:"name"`
	ThemeVersion string                   `json:"theme_version"`
	APIVersion   int                      `json:"api_version"`
	Live         bool                     `json:"live"`
	Settings     []HelpCenterThemeSetting `json:"settings"`
	Values       map[string]any           `json:"values"`
	ContentHash  string                   `json:"content_hash"`
	FileCount    int32                    `json:"file_count"`
	SizeBytes    int32                    `json:"size_bytes"`
	CreatedAt    time.Time                `json:"created_at"`
	UpdatedAt    time.Time                `json:"updated_at"`
}

/*
NewHelpCenterTheme shapes a theme version, live when the help center
wears it.
*/
func NewHelpCenterTheme(t models.HelpCenterTheme, live bool) HelpCenterTheme {
	var m themes.Manifest
	_ = json.Unmarshal(t.Manifest, &m)
	var values map[string]any
	_ = json.Unmarshal(t.Settings, &values)
	out := HelpCenterTheme{
		ID: models.PublicID(models.PrefixHelpCenterTheme, t.ID), Object: "help_center_theme",
		HelpCenterID: models.PublicID(models.PrefixHelpCenter, t.HelpCenterID), Version: t.Version, Name: t.Name,
		ThemeVersion: m.Version, APIVersion: m.APIVersion, Live: live, Settings: []HelpCenterThemeSetting{},
		Values: m.SettingValues(values), ContentHash: t.ContentHash, FileCount: t.FileCount, SizeBytes: t.SizeBytes,
		CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt,
	}
	for _, s := range m.Settings {
		out.Settings = append(out.Settings, HelpCenterThemeSetting(s))
	}
	return out
}
