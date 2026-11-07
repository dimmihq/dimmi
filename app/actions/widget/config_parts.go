/*
|--------------------------------------------------------------------------
| Widget Config Parts
|--------------------------------------------------------------------------
|
| Who a messenger speaks for, what it links to and which features it turns
| on. Each part says only what a browser needs to draw it, so nothing here
| names a member or a visitor.
|
*/

package widget

/*
ConfigCompany is who the messenger speaks for.
*/
type ConfigCompany struct {
	Name    string  `json:"name"`
	LogoURL *string `json:"logo_url"`
}

/*
ConfigPrivacy is the privacy notice above the composer.
*/
type ConfigPrivacy struct {
	Enabled bool    `json:"enabled"`
	URL     *string `json:"url"`
}

/*
ConfigHomeLink is one link on the home screen.
*/
type ConfigHomeLink struct {
	Title string `json:"title"`
	URL   string `json:"url"`
	Icon  string `json:"icon"`
}

/*
ConfigDevContext says what the widget captures for the team.
*/
type ConfigDevContext struct {
	Console bool `json:"console"`
	Network bool `json:"network"`
}

/*
ConfigFeatures says what the widget can do.
*/
type ConfigFeatures struct {
	Attachments             bool             `json:"attachments"`
	MaxAttachmentMB         int32            `json:"max_attachment_mb"`
	HelpTab                 bool             `json:"help_tab"`
	AIAgent                 bool             `json:"ai_agent"`
	CollectEmail            string           `json:"collect_email"`
	RequireEmailWhenOffline bool             `json:"require_email_when_offline"`
	AllowAnonymous          bool             `json:"allow_anonymous"`
	IdentityMode            string           `json:"identity_mode"`
	IntercomShim            bool             `json:"intercom_shim"`
	DevContext              ConfigDevContext `json:"dev_context"`
}
