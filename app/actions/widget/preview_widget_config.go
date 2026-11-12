/*
|--------------------------------------------------------------------------
| Preview Widget Config
|--------------------------------------------------------------------------
|
| What a widget would tell a browser if a change were saved, without saving
| it, so the settings page can show the messenger as it will look while
| someone is still choosing. The change goes through the same rules as a
| save, so a value the save would refuse is refused here too, and the
| config is built the same way a visitor's is, so the preview never drifts
| from what goes live. Nothing is cached and nothing is written.
|
*/

package widget

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/widgetstrings"
)

/*
PreviewWidgetConfigInput is who previews which widget, the change they are
considering, the language to show it in and the base address of this
install.
*/
type PreviewWidgetConfigInput struct {
	Membership authz.Membership
	WidgetID   ids.UUID
	Change     SettingsChange
	Locale     string
	BaseURL    string
}

/*
PreviewWidgetConfig builds a widget's config with a change applied, leaving
the widget as it is.

Rules:
  - The caller needs settings:read, the same as reading the widget.
  - The change must pass every rule a save checks.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], the save's
validation errors, or a wrapped database error.
*/
type PreviewWidgetConfig struct {
	DB   *database.DB
	Show *ShowWidgetConfig
}

/*
Handle returns the config the widget would serve with the change saved.
*/
func (a *PreviewWidgetConfig) Handle(ctx context.Context, in PreviewWidgetConfigInput) (Config, error) {
	if !canRead(in.Membership) {
		return Config{}, exceptions.ErrForbidden
	}

	ws := in.Membership.Workspace

	var stored models.WidgetSetting

	err := a.DB.Transaction(tenancy.With(ctx, ws.ID.String()), func(ctx context.Context, tx *database.Tx) error {
		var err error
		stored, err = models.New(tx).FindWidgetSettings(ctx, models.FindWidgetSettingsParams{
			WorkspaceID: ws.ID, ID: in.WidgetID,
		})

		return err
	})
	if errors.Is(err, database.ErrNotFound) {
		return Config{}, exceptions.ErrNotFound
	}

	if err != nil {
		return Config{}, fmt.Errorf("find widget: %w", err)
	}

	params, err := changedParams(stored, in.Change)
	if err != nil {
		return Config{}, err
	}

	draft := withParams(stored, params)
	locale := a.Show.Strings.Negotiate(widgetstrings.Request{
		Requested: in.Locale, Forced: forcedLocale(draft), Default: "en",
	})
	show := ShowWidgetConfigInput{
		Resolved: Resolved{Widget: draft, Workspace: ws}, Locale: in.Locale, BaseURL: in.BaseURL,
	}

	return a.Show.build(tenancy.With(ctx, ws.ID.String()), show, locale)
}

/*
withParams returns the widget as the saved values would leave it.
*/
func withParams(w models.WidgetSetting, p models.SaveWidgetSettingsParams) models.WidgetSetting {
	w.Name, w.Enabled, w.Appearance, w.Texts, w.Locale = p.Name, p.Enabled, p.Appearance, p.Texts, p.Locale
	w.ReplyTime, w.ShowTeamAvatars, w.ShowAvailability = p.ReplyTime, p.ShowTeamAvatars, p.ShowAvailability
	w.AllowedOrigins, w.AllowAnonymous, w.CollectEmail = p.AllowedOrigins, p.AllowAnonymous, p.CollectEmail
	w.RequireEmailWhenOffline, w.AttachmentsEnabled = p.RequireEmailWhenOffline, p.AttachmentsEnabled
	w.MaxAttachmentMb, w.HelpCenterID, w.HelpTabEnabled = p.MaxAttachmentMb, p.HelpCenterID, p.HelpTabEnabled
	w.ChangelogTabEnabled, w.IntercomShim = p.ChangelogTabEnabled, p.IntercomShim
	w.NativeAppsEnabled, w.ShowBranding = p.NativeAppsEnabled, p.ShowBranding
	w.HomeLinks, w.TextsByLocale = p.HomeLinks, p.TextsByLocale

	return w
}
