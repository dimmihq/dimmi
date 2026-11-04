/*
|--------------------------------------------------------------------------
| Widget Settings Requests
|--------------------------------------------------------------------------
|
| Inputs of the dashboard's widget routes. Every field of an update is
| optional, and a missing one stays as it is. The appearance and texts are
| open objects here because their keys are checked one by one in the action,
| which names the exact key a form shows its error beside.
|
*/

package requests

import "github.com/dimmihq/dimmi/app/actions/widget"

/*
ShowWidgetSettingsRequest is the input of the routes on one widget.
*/
type ShowWidgetSettingsRequest struct {
	Widget string `path:"widget"`
}

/*
WidgetSettingsBody is a widget's settings as a request sends them.
*/
type WidgetSettingsBody struct {
	Name                    *string        `json:"name,omitempty"`
	InboxID                 *string        `json:"inbox_id,omitempty"`
	Enabled                 *bool          `json:"enabled,omitempty"`
	Appearance              map[string]any `json:"appearance,omitempty"`
	Texts                   map[string]any `json:"texts,omitempty"`
	Locale                  *string        `json:"locale,omitempty"`
	ReplyTime               *string        `json:"reply_time,omitempty"`
	ShowTeamAvatars         *bool          `json:"show_team_avatars,omitempty"`
	ShowAvailability        *bool          `json:"show_availability,omitempty"`
	AllowedOrigins          []string       `json:"allowed_origins,omitempty"`
	AllowAnonymous          *bool          `json:"allow_anonymous,omitempty"`
	CollectEmail            *string        `json:"collect_email,omitempty"`
	RequireEmailWhenOffline *bool          `json:"require_email_when_offline,omitempty"`
	AttachmentsEnabled      *bool          `json:"attachments_enabled,omitempty"`
	MaxAttachmentMB         *int32         `json:"max_attachment_mb,omitempty"`
	HelpCenterID            *string        `json:"help_center_id,omitempty"`
	HelpTabEnabled          *bool          `json:"help_tab_enabled,omitempty"`
	ChangelogTabEnabled     *bool          `json:"changelog_tab_enabled,omitempty"`
	IntercomShim            *bool          `json:"intercom_shim,omitempty"`
	NativeAppsEnabled       *bool          `json:"native_apps_enabled,omitempty"`
	ShowBranding            *bool          `json:"show_branding,omitempty"`
}

/*
Change returns the settings change the body asks for, given which fields the
raw body sends. The inbox and help center ids are decoded by the controller.
*/
func (b WidgetSettingsBody) Change(raw []byte) widget.SettingsChange {
	sent := Sent(raw)

	return widget.SettingsChange{
		Name: b.Name, Enabled: b.Enabled, Appearance: b.Appearance, Texts: b.Texts, Locale: b.Locale,
		ReplyTime: b.ReplyTime, ShowTeamAvatars: b.ShowTeamAvatars, ShowAvailability: b.ShowAvailability,
		AllowedOrigins: b.AllowedOrigins, AllowedOriginsSent: sent["allowed_origins"],
		AllowAnonymous: b.AllowAnonymous, CollectEmail: b.CollectEmail,
		RequireEmailWhenOffline: b.RequireEmailWhenOffline, AttachmentsEnabled: b.AttachmentsEnabled,
		MaxAttachmentMB: b.MaxAttachmentMB, HelpCenterSent: sent["help_center_id"], HelpTabEnabled: b.HelpTabEnabled,
		ChangelogTabEnabled: b.ChangelogTabEnabled, IntercomShim: b.IntercomShim,
		NativeAppsEnabled: b.NativeAppsEnabled, ShowBranding: b.ShowBranding, ShowBrandingSent: sent["show_branding"],
	}
}

/*
StoreWidgetSettingsRequest is the input of POST /widget-settings.
*/
type StoreWidgetSettingsRequest struct {
	Body    WidgetSettingsBody
	RawBody []byte
}

/*
UpdateWidgetSettingsRequest is the input of PATCH /widget-settings/{widget}.
*/
type UpdateWidgetSettingsRequest struct {
	Widget  string `path:"widget"`
	Body    WidgetSettingsBody
	RawBody []byte
}
