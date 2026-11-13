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

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/actions/identity"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/widgetstrings"
)

/*
HoneypotField names the form field real visitors never fill.
*/
const HoneypotField = "website"

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
PollingInterval is how often a browser without a stream polls. Surfaces
decides the messenger's rules for a visitor nobody knows.
*/
type ShowWidgetConfig struct {
	Directory       *Directory
	Surfaces        *Surfaces
	Branding        *ResolveBranding
	Strings         *widgetstrings.Strings
	Openers         Openers
	PollingInterval int64
}

/*
Openers says whether a workspace greets visitors as they open a chat.
*/
type Openers interface {
	/*
		HasOpener reports whether a live workflow opens conversations in
		the workspace.
	*/
	HasOpener(ctx context.Context, workspace ids.UUID) (bool, error)
}

/*
OpenerFunc is an Openers made of one function, so an install can answer
from whatever it has by the time a visitor asks rather than from what
existed when the messenger was built.
*/
type OpenerFunc func(ctx context.Context, workspace ids.UUID) (bool, error)

/*
HasOpener asks the function.
*/
func (f OpenerFunc) HasOpener(ctx context.Context, workspace ids.UUID) (bool, error) {
	return f(ctx, workspace)
}

/*
Handle returns the config for the input and the language it resolved to.
*/
func (a *ShowWidgetConfig) Handle(ctx context.Context, in ShowWidgetConfigInput) (Config, error) {
	widget := in.Resolved.Widget
	locale := a.Strings.Negotiate(widgetstrings.Request{
		Requested: in.Locale, Forced: forcedLocale(widget), Default: "en",
	})
	key := fmt.Sprintf("%s:%d:%s:%s:%s", widget.ID, widget.ConfigVersion, locale, in.BaseURL, companyKey(in.Resolved))

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
company is who the messenger speaks for: the workspace's name and its
logo, addressed on this server so a customer's site can load it.
*/
func company(r Resolved, baseURL string) ConfigCompany {
	out := ConfigCompany{Name: r.Workspace.Name}
	if logo := r.Workspace.LogoURL(); logo != nil {
		url := baseURL + *logo
		out.LogoURL = &url
	}

	return out
}

/*
companyKey tells built configs apart when the workspace's name or logo
changes, which does not move the widget's config version.
*/
func companyKey(r Resolved) string {
	logo := ""
	if r.Workspace.LogoPath != nil {
		logo = *r.Workspace.LogoPath
	}

	return r.Workspace.Name + "|" + logo
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

	byLocale, err := TextsByLocale(widget)
	if err != nil {
		return Config{}, err
	}

	texts, err := a.Strings.ResolveWith(locale, "en", widget.Texts, byLocale)
	if err != nil {
		return Config{}, err
	}

	surface, err := a.surface(ctx, widget)
	if err != nil {
		return Config{}, err
	}

	features := configFeatures(r)

	features.Opener, err = a.opens(ctx, r)
	if err != nil {
		return Config{}, err
	}

	return Config{
		Surface: surface,
		Object:  "widget_config", AppID: models.PublicID(models.PrefixWidget, widget.ID),
		ConfigVersion: widget.ConfigVersion, Company: company(r, in.BaseURL),
		Appearance: configAppearance(appearance), Texts: texts, Locale: locale, Dir: widgetstrings.Direction(locale),
		Locales: a.Strings.Locales(), Tabs: configTabs(widget), OpenTo: "home",
		ShowTeamAvatars: widget.ShowTeamAvatars, ShowAvailability: widget.ShowAvailability, CustomerStatus: true,
		DelayNotice: true, HomeLinks: HomeLinks(widget), Features: features, Branding: branding,
		Realtime: ConfigRealtime{
			URL: in.BaseURL + "/connection/uni_sse", TokenURL: in.BaseURL + "/widget/v1/realtime-tokens",
			PollingIntervalMS: a.PollingInterval,
		},
		HoneypotField: HoneypotField,
	}, nil
}

/*
opens reports whether a workflow greets this workspace's visitors as
they open a chat. A workspace without one is the common case, and its
messenger keeps starting conversations from the first message.
*/
func (a *ShowWidgetConfig) opens(ctx context.Context, r Resolved) (bool, error) {
	if a.Openers == nil {
		return false, nil
	}

	return a.Openers.HasOpener(ctx, r.Workspace.ID)
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
configFeatures says what the widget can do. Anonymous visitors are
allowed only when the widget allows them and sign-in is not required.
*/
func configFeatures(resolved Resolved) ConfigFeatures {
	widget := resolved.Widget

	return ConfigFeatures{
		Attachments: widget.AttachmentsEnabled, MaxAttachmentMB: widget.MaxAttachmentMb,
		HelpTab:      widget.HelpTabEnabled && widget.HelpCenterID != nil,
		CollectEmail: widget.CollectEmail, RequireEmailWhenOffline: widget.RequireEmailWhenOffline,
		AllowAnonymous: widget.AllowAnonymous && resolved.IdentityMode != identity.ModeRequired,
		IdentityMode:   resolved.IdentityMode, IntercomShim: widget.IntercomShim,
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

/*
surface is the anonymous outcome of the messenger's rules, nil when it
has none.
*/
func (a *ShowWidgetConfig) surface(ctx context.Context, widget models.WidgetSetting) (*ConfigSurface, error) {
	if a.Surfaces == nil {
		return nil, nil
	}

	anonymous, err := a.Surfaces.Anonymous(ctx, widget)
	if err != nil || anonymous == nil {
		return nil, err
	}

	return &ConfigSurface{Version: widget.ConfigVersion, Anonymous: anonymous}, nil
}
