/*
|--------------------------------------------------------------------------
| Widget Settings Resource
|--------------------------------------------------------------------------
|
| A widget as its settings page shows it, with every default filled in, so a
| form always has a value to show. Internal ids stay out: the app id is the
| widget's public face.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/widget"
	"github.com/dimmihq/dimmi/app/models"
)

/*
WidgetSettingsLinks are the addresses of what belongs to a widget.
*/
type WidgetSettingsLinks struct {
	Self    string `json:"self"`
	Install string `json:"install"`
	Config  string `json:"config"`
}

/*
WidgetSettings is one widget as its settings page shows it.
*/
type WidgetSettings struct {
	ID                      string              `json:"id"`
	Object                  string              `json:"object"`
	Name                    string              `json:"name"`
	Enabled                 bool                `json:"enabled"`
	Inbox                   Named               `json:"inbox"`
	ChannelConnectionID     string              `json:"channel_connection_id"`
	Appearance              widget.Appearance   `json:"appearance"`
	Texts                   map[string]any      `json:"texts"`
	Locale                  string              `json:"locale"`
	ReplyTime               string              `json:"reply_time"`
	ShowTeamAvatars         bool                `json:"show_team_avatars"`
	ShowAvailability        bool                `json:"show_availability"`
	AllowedOrigins          []string            `json:"allowed_origins"`
	AllowAnonymous          bool                `json:"allow_anonymous"`
	CollectEmail            string              `json:"collect_email"`
	RequireEmailWhenOffline bool                `json:"require_email_when_offline"`
	AttachmentsEnabled      bool                `json:"attachments_enabled"`
	MaxAttachmentMB         int32               `json:"max_attachment_mb"`
	NativeAppsEnabled       bool                `json:"native_apps_enabled"`
	HelpCenterID            *string             `json:"help_center_id"`
	HelpTabEnabled          bool                `json:"help_tab_enabled"`
	ChangelogTabEnabled     bool                `json:"changelog_tab_enabled"`
	IntercomShim            bool                `json:"intercom_shim"`
	ShowBranding            *bool               `json:"show_branding"`
	Branding                widget.Branding     `json:"branding"`
	ConfigVersion           int64               `json:"config_version"`
	LastBootAt              *time.Time          `json:"last_boot_at"`
	LastBootOrigin          *string             `json:"last_boot_origin"`
	CreatedAt               time.Time           `json:"created_at"`
	UpdatedAt               time.Time           `json:"updated_at"`
	Links                   WidgetSettingsLinks `json:"links"`
}

/*
NewWidgetSettings shapes the widget at the base address of this install.
*/
func NewWidgetSettings(row widget.SettingsRow, base string) WidgetSettings {
	w := row.Widget
	public := models.PublicID(models.PrefixWidget, w.ID)

	return WidgetSettings{
		ID: public, Object: "widget_settings", Name: w.Name, Enabled: w.Enabled,
		Inbox: Named{
			ID: models.PublicID(models.PrefixInbox, row.Inbox.ID), Object: "inbox", Name: row.Inbox.Name,
		},
		ChannelConnectionID: models.PublicID(models.PrefixChannelConnection, w.ChannelConnectionID),
		Appearance:          row.Appearance, Texts: row.Texts, Locale: w.Locale, ReplyTime: w.ReplyTime,
		ShowTeamAvatars: w.ShowTeamAvatars, ShowAvailability: w.ShowAvailability, AllowedOrigins: w.AllowedOrigins,
		AllowAnonymous: w.AllowAnonymous, CollectEmail: w.CollectEmail,
		RequireEmailWhenOffline: w.RequireEmailWhenOffline, AttachmentsEnabled: w.AttachmentsEnabled,
		MaxAttachmentMB: w.MaxAttachmentMb, NativeAppsEnabled: w.NativeAppsEnabled,
		HelpCenterID:   helpCenterID(w.HelpCenterID),
		HelpTabEnabled: w.HelpTabEnabled, ChangelogTabEnabled: w.ChangelogTabEnabled, IntercomShim: w.IntercomShim,
		ShowBranding: w.ShowBranding, Branding: row.Branding, ConfigVersion: w.ConfigVersion, LastBootAt: w.LastBootAt,
		LastBootOrigin: w.LastBootOrigin, CreatedAt: w.CreatedAt, UpdatedAt: w.UpdatedAt,
		Links: WidgetSettingsLinks{
			Self:    base + "/api/v1/widget-settings/" + public,
			Install: base + "/api/v1/widget-settings/" + public + "/install",
			Config:  base + "/widget/v1/apps/" + public + "/config",
		},
	}
}

/*
helpCenterID returns the public id of a linked help center.
*/
func helpCenterID(id *ids.UUID) *string {
	if id == nil {
		return nil
	}

	public := ids.Encode("hc", *id)

	return &public
}
