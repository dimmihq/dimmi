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
