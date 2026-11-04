/*
|--------------------------------------------------------------------------
| Widget Config
|--------------------------------------------------------------------------
|
| The one response every page load makes, and often the only one: how the
| messenger looks, what it says and what it can do. It is public and cached
| by CDNs, so it carries nothing about any visitor, no asset ids and no
| member's details. A built config is kept per version and language, and
| the language is normalized first, so random values cannot grow the cache.
|
*/

package widget

import (
	"context"
	"fmt"
	"slices"

	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/widgetstrings"
)

/*
HoneypotField names the form field real visitors never fill.
*/
const HoneypotField = "website"

/*
ConfigCompany is who the messenger speaks for.
*/
type ConfigCompany struct {
	Name    string  `json:"name"`
	LogoURL *string `json:"logo_url"`
}

/*
ConfigGradient is a gradient as a page draws it.
*/
type ConfigGradient struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Angle int    `json:"angle"`
}

/*
ConfigBackground is what fills the home hero and header.
*/
type ConfigBackground struct {
	Type     string          `json:"type"`
	Gradient *ConfigGradient `json:"gradient,omitempty"`
	ImageURL *string         `json:"image_url,omitempty"`
	Fade     bool            `json:"fade"`
}

/*
ConfigLauncher is how the launcher looks.
*/
type ConfigLauncher struct {
	Style    string  `json:"style"`
	Shape    string  `json:"shape"`
	Label    string  `json:"label"`
	Icon     string  `json:"icon"`
	ImageURL *string `json:"image_url"`
}

/*
ConfigAppearance is the look, with ids resolved to addresses and the ink
that reads on each brand surface.
*/
type ConfigAppearance struct {
	PrimaryColor string           `json:"primary_color"`
	HeaderColor  string           `json:"header_color"`
	ButtonColor  string           `json:"button_color"`
	HeaderInk    string           `json:"header_ink"`
	ButtonInk    string           `json:"button_ink"`
	Background   ConfigBackground `json:"background"`
	Theme        string           `json:"theme"`
	Position     string           `json:"position"`
	PaddingX     int              `json:"padding_x"`
	PaddingY     int              `json:"padding_y"`
	Launcher     ConfigLauncher   `json:"launcher"`
	Font         string           `json:"font"`
	Sound        bool             `json:"sound"`
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
	IntercomShim            bool             `json:"intercom_shim"`
	DevContext              ConfigDevContext `json:"dev_context"`
}

/*
ConfigRealtime says where live updates come from and how often to poll when
they cannot.
*/
type ConfigRealtime struct {
	URL               string `json:"url"`
	TokenURL          string `json:"token_url"`
	PollingIntervalMS int64  `json:"polling_interval_ms"`
}

/*
ConfigSurface carries the rule for showing the launcher, in the audience
language, for the browser to evaluate on each page.
*/
type ConfigSurface struct {
	Version   int64          `json:"version"`
	Dynamic   bool           `json:"dynamic"`
	Anonymous map[string]any `json:"anonymous"`
}

/*
Config is what a widget tells a browser.
*/
type Config struct {
	Object           string            `json:"object"`
	AppID            string            `json:"app_id"`
	ConfigVersion    int64             `json:"config_version"`
	Company          ConfigCompany     `json:"company"`
	Appearance       ConfigAppearance  `json:"appearance"`
	Texts            map[string]string `json:"texts"`
	Locale           string            `json:"locale"`
	Dir              string            `json:"dir"`
	Locales          []string          `json:"locales"`
	Tabs             []string          `json:"tabs"`
	OpenTo           string            `json:"open_to"`
	ShowTeamAvatars  bool              `json:"show_team_avatars"`
	ShowAvailability bool              `json:"show_availability"`
	CustomerStatus   bool              `json:"customer_status"`
	DelayNotice      bool              `json:"delay_notice"`
	Privacy          ConfigPrivacy     `json:"privacy"`
	HomeLinks        []ConfigHomeLink  `json:"home_links"`
	Surface          *ConfigSurface    `json:"surface,omitempty"`
	Features         ConfigFeatures    `json:"features"`
	Branding         Branding          `json:"branding"`
	Realtime         ConfigRealtime    `json:"realtime"`
	HoneypotField    string            `json:"honeypot_field"`
}

/*
ShowWidgetConfigInput is the widget, the language the visitor asked for and
the base address the visitor reached this install on.
*/
type ShowWidgetConfigInput struct {
	Resolved Resolved
	Locale   string
	BaseURL  string
}

/*
ShowWidgetConfig builds what a widget tells a browser. Directory holds built
configs, Branding decides the badge, Strings holds the built-in words and
PollingInterval is how often a browser without a stream polls.
*/
type ShowWidgetConfig struct {
	Directory       *Directory
	Branding        *ResolveBranding
	Strings         *widgetstrings.Strings
	PollingInterval int64
}

/*
Handle returns the config for the input and the language it resolved to.
*/
func (a *ShowWidgetConfig) Handle(ctx context.Context, in ShowWidgetConfigInput) (Config, error) {
	widget := in.Resolved.Widget
	locale := a.Strings.Negotiate(widgetstrings.Request{
		Requested: in.Locale, Forced: forcedLocale(widget), Default: "en",
	})
	key := fmt.Sprintf("%s:%d:%s:%s", widget.ID, widget.ConfigVersion, locale, in.BaseURL)

	if config, ok := a.Directory.config(key); ok {
		return config, nil
	}

	config, err := a.build(tenancy.With(ctx, widget.WorkspaceID.String()), in, locale)
	if err != nil {
		return config, err
	}

	a.Directory.keepConfig(key, config)

	return config, nil
}

/*
forcedLocale returns the widget's fixed language, empty when it follows the
browser.
*/
func forcedLocale(widget models.WidgetSetting) string {
	if widget.Locale == LocaleAuto {
		return ""
	}

	return widget.Locale
}

/*
build assembles the config.
*/
func (a *ShowWidgetConfig) build(ctx context.Context, in ShowWidgetConfigInput, locale string) (Config, error) {
	r, widget := in.Resolved, in.Resolved.Widget

	appearance, err := ResolveAppearance(widget.Appearance)
	if err != nil {
		return Config{}, err
	}

	branding, err := a.Branding.Handle(ctx, r.Workspace, widget.ShowBranding)
	if err != nil {
		return Config{}, err
	}

	texts, err := a.Strings.Resolve(locale, "en", widget.Texts)
	if err != nil {
		return Config{}, err
	}

	return Config{
		Object: "widget_config", AppID: models.PublicID(models.PrefixWidget, widget.ID),
		ConfigVersion: widget.ConfigVersion, Company: ConfigCompany{Name: r.Workspace.Name},
		Appearance: configAppearance(appearance), Texts: texts, Locale: locale, Dir: widgetstrings.Direction(locale),
		Locales: a.Strings.Locales(), Tabs: configTabs(widget), OpenTo: "home",
		ShowTeamAvatars: widget.ShowTeamAvatars, ShowAvailability: widget.ShowAvailability, CustomerStatus: true,
		DelayNotice: true, HomeLinks: []ConfigHomeLink{}, Features: configFeatures(widget), Branding: branding,
		Realtime: ConfigRealtime{
			URL: in.BaseURL + "/connection/uni_sse", TokenURL: in.BaseURL + "/widget/v1/realtime-tokens",
			PollingIntervalMS: a.PollingInterval,
		},
		HoneypotField: HoneypotField,
	}, nil
}

/*
configTabs returns the tabs that are on, in their order.
*/
func configTabs(widget models.WidgetSetting) []string {
	tabs := []string{"home", "messages"}
	if widget.HelpTabEnabled && widget.HelpCenterID != nil {
		tabs = append(tabs, "help")
	}

	if widget.ChangelogTabEnabled && widget.HelpCenterID != nil {
		tabs = append(tabs, "changelog")
	}

	return slices.Clip(tabs)
}

/*
configFeatures says what the widget can do.
*/
func configFeatures(widget models.WidgetSetting) ConfigFeatures {
	return ConfigFeatures{
		Attachments: widget.AttachmentsEnabled, MaxAttachmentMB: widget.MaxAttachmentMb,
		HelpTab:      widget.HelpTabEnabled && widget.HelpCenterID != nil,
		CollectEmail: widget.CollectEmail, RequireEmailWhenOffline: widget.RequireEmailWhenOffline,
		AllowAnonymous: widget.AllowAnonymous, IntercomShim: widget.IntercomShim,
	}
}

/*
configAppearance resolves the look into what a page draws.
*/
func configAppearance(a Appearance) ConfigAppearance {
	header, button := a.PrimaryColor, a.PrimaryColor
	if a.HeaderColor != nil {
		header = *a.HeaderColor
	}

	if a.ButtonColor != nil {
		button = *a.ButtonColor
	}

	out := ConfigAppearance{
		PrimaryColor: a.PrimaryColor, HeaderColor: header, ButtonColor: button, HeaderInk: inkOn(header),
		ButtonInk: inkOn(button), Theme: a.Theme, Position: a.Position, PaddingX: a.PaddingX, PaddingY: a.PaddingY,
		Font: a.Font, Sound: a.Sound, Background: ConfigBackground{Type: a.Background.Type, Fade: a.Background.Fade},
		Launcher: ConfigLauncher{
			Style: a.Launcher.Style, Shape: a.Launcher.Shape, Label: a.Launcher.Label, Icon: a.Launcher.Icon,
		},
	}
	if g := a.Background.Gradient; g != nil {
		out.Background.Gradient = &ConfigGradient{From: g.From, To: g.To, Angle: g.Angle}
	}

	return out
}
