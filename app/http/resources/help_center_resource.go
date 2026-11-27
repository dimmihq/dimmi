/*
|--------------------------------------------------------------------------
| Help Center Resources
|--------------------------------------------------------------------------
|
| How help centers and their collections read over the API. The public
| address is spelled out, so the dashboard links to exactly what readers
| get: the verified custom domain, named on its own too, once there is
| one.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/app/actions/helpcenters"
	"github.com/dimmihq/dimmi/app/models"
)

/*
HelpCenter is one help center and where readers reach it. Locale and
DefaultLocale both name the language its articles are written in;
Locales lists every language readers may pick, the default first.
*/
type HelpCenter struct {
	ID              string                         `json:"id"`
	Object          string                         `json:"object"`
	Name            string                         `json:"name"`
	Subdomain       string                         `json:"subdomain"`
	URL             string                         `json:"url"`
	CustomDomain    *string                        `json:"custom_domain"`
	Locale          string                         `json:"locale"`
	DefaultLocale   string                         `json:"default_locale"`
	Locales         []string                       `json:"locales"`
	IsDefault       bool                           `json:"is_default"`
	Status          string                         `json:"status"`
	IndexingEnabled bool                           `json:"indexing_enabled"`
	AskAIEnabled    bool                           `json:"ask_ai_enabled"`
	Settings        helpcenters.HelpCenterSettings `json:"settings"`
	ThemeID         *string                        `json:"theme_id"`
	CreatedAt       time.Time                      `json:"created_at"`
	UpdatedAt       time.Time                      `json:"updated_at"`
}

/*
NewHelpCenter shapes a help center reachable at the URL.
*/
func NewHelpCenter(h models.HelpCenter, url string) HelpCenter {
	var theme *string
	if h.ThemeID != nil {
		theme = new(models.PublicID(models.PrefixHelpCenterTheme, *h.ThemeID))
	}
	return HelpCenter{
		ThemeID: theme,
		ID:      models.PublicID(models.PrefixHelpCenter, h.ID), Object: "help_center", Name: h.Name,
		Subdomain: h.Subdomain, URL: url, CustomDomain: h.CustomDomain, Locale: h.Locale, DefaultLocale: h.Locale,
		Locales: helpcenters.LocalesOf(h), IsDefault: h.IsDefault,
		Status: h.Status, IndexingEnabled: h.IndexingEnabled, AskAIEnabled: h.AskAiEnabled,
		Settings: helpcenters.SettingsOf(h), CreatedAt: h.CreatedAt, UpdatedAt: h.UpdatedAt,
	}
}

/*
Collection is one collection of a help center.
*/
type Collection struct {
	ID           string    `json:"id"`
	Object       string    `json:"object"`
	HelpCenterID string    `json:"help_center_id"`
	ParentID     *string   `json:"parent_id"`
	Name         string    `json:"name"`
	Slug         string    `json:"slug"`
	Description  *string   `json:"description"`
	Icon         *string   `json:"icon"`
	Position     int32     `json:"position"`
	AudienceID   *string   `json:"audience_id"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

/*
NewCollection shapes a collection.
*/
func NewCollection(c models.Collection) Collection {
	out := Collection{
		ID: models.PublicID(models.PrefixCollection, c.ID), Object: "collection",
		HelpCenterID: models.PublicID(models.PrefixHelpCenter, c.HelpCenterID), Name: c.Name, Slug: c.Slug,
		Description: c.Description, Icon: c.Icon, Position: c.Position, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
	}
	if c.ParentID != nil {
		out.ParentID = new(models.PublicID(models.PrefixCollection, *c.ParentID))
	}
	if c.AudienceID != nil {
		out.AudienceID = new(models.PublicID(models.PrefixAudience, *c.AudienceID))
	}
	return out
}

/*
NewCollections shapes a list of collections.
*/
func NewCollections(rows []models.Collection) []Collection {
	out := make([]Collection, len(rows))
	for i, r := range rows {
		out[i] = NewCollection(r)
	}
	return out
}
