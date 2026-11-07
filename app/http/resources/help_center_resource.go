/*
|--------------------------------------------------------------------------
| Help Center Resources
|--------------------------------------------------------------------------
|
| How help centers and their collections read over the API. The public
| address is spelled out, so the dashboard links to exactly what readers
| get.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/app/actions/helpcenters"
	"github.com/dimmihq/dimmi/app/models"
)

/*
HelpCenter is one help center and where readers reach it.
*/
type HelpCenter struct {
	ID              string                         `json:"id"`
	Object          string                         `json:"object"`
	Name            string                         `json:"name"`
	Subdomain       string                         `json:"subdomain"`
	URL             string                         `json:"url"`
	Locale          string                         `json:"locale"`
	IsDefault       bool                           `json:"is_default"`
	Status          string                         `json:"status"`
	IndexingEnabled bool                           `json:"indexing_enabled"`
	AskAIEnabled    bool                           `json:"ask_ai_enabled"`
	Settings        helpcenters.HelpCenterSettings `json:"settings"`
	CreatedAt       time.Time                      `json:"created_at"`
	UpdatedAt       time.Time                      `json:"updated_at"`
}

/*
NewHelpCenter shapes a help center reachable at the URL.
*/
func NewHelpCenter(h models.HelpCenter, url string) HelpCenter {
	return HelpCenter{
		ID: models.PublicID(models.PrefixHelpCenter, h.ID), Object: "help_center", Name: h.Name,
		Subdomain: h.Subdomain, URL: url, Locale: h.Locale, IsDefault: h.IsDefault, Status: h.Status,
		IndexingEnabled: h.IndexingEnabled, AskAIEnabled: h.AskAiEnabled, Settings: helpcenters.SettingsOf(h),
		CreatedAt: h.CreatedAt, UpdatedAt: h.UpdatedAt,
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
