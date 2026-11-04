/*
|--------------------------------------------------------------------------
| Update Widget Settings
|--------------------------------------------------------------------------
|
| Every change to a widget goes through here: check it, write it, move the
| widget's version on and tell every replica to drop its copy, so live pages
| pick it up within a minute and who-may-use-it changes apply at once. Only
| what the caller sends changes, so two people editing different fields
| never undo each other. The fields that decide who may use a widget are
| recorded in the audit log with their values.
|
*/

package widget

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/support/str"

	"github.com/dimmihq/dimmi/app/auth/authz"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
SettingsChange is what a caller sends; a nil field stays as it is. Show
branding has a meaning for null, so ShowBrandingSent tells null from
absent, and AllowedOrigins from nil likewise by AllowedOriginsSent.
*/
type SettingsChange struct {
	Name                    *string
	InboxID                 *ids.UUID
	Enabled                 *bool
	Appearance              map[string]any
	Texts                   map[string]any
	Locale                  *string
	ReplyTime               *string
	ShowTeamAvatars         *bool
	ShowAvailability        *bool
	AllowedOrigins          []string
	AllowedOriginsSent      bool
	AllowAnonymous          *bool
	CollectEmail            *string
	RequireEmailWhenOffline *bool
	AttachmentsEnabled      *bool
	MaxAttachmentMB         *int32
	HelpCenterID            *ids.UUID
	HelpCenterSent          bool
	HelpTabEnabled          *bool
	ChangelogTabEnabled     *bool
	IntercomShim            *bool
	NativeAppsEnabled       *bool
	ShowBranding            *bool
	ShowBrandingSent        bool
}

/*
UpdateWidgetSettingsInput is who changes which widget and how.
*/
type UpdateWidgetSettingsInput struct {
	Membership authz.Membership
	WidgetID   ids.UUID
	Change     SettingsChange
}

/*
UpdateWidgetSettings changes a widget's settings.

Rules:
  - The caller needs widgets:manage.
  - A workspace that never paid on the cloud cannot hide the badge.
  - A help tab needs a linked help center.

Events: [appevents.WidgetSettingsUpdated] when anything changed.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound],
[exceptions.ErrBrandingLocked], a field refusal from
[exceptions.WidgetField], or a wrapped database error.
*/
type UpdateWidgetSettings struct {
	DB       *database.DB
	Events   *events.Dispatcher
	Clock    clock.Clock
	Branding *ResolveBranding
	Bust     *Directory
}

/*
Handle applies the change and returns the widget as it now is.
*/
func (a *UpdateWidgetSettings) Handle(ctx context.Context, in UpdateWidgetSettingsInput) (models.WidgetSetting, error) {
	if !in.Membership.Can("widgets:manage") {
		return models.WidgetSetting{}, exceptions.ErrForbidden
	}

	if err := a.checkBranding(ctx, in); err != nil {
		return models.WidgetSetting{}, err
	}

	var out models.WidgetSetting

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error

		out, err = a.apply(ctx, tx, in)

		return err
	})
	if err != nil {
		return out, err
	}

	a.Bust.Forget(ctx, in.WidgetID)

	return out, nil
}

/*
checkBranding refuses hiding the badge while it is locked.
*/
func (a *UpdateWidgetSettings) checkBranding(ctx context.Context, in UpdateWidgetSettingsInput) error {
	hide := in.Change.ShowBranding
	if hide == nil || *hide {
		return nil
	}

	branding, err := a.Branding.Handle(ctx, in.Membership.Workspace, nil)
	if err != nil {
		return err
	}

	if branding.Locked {
		return exceptions.ErrBrandingLocked
	}

	return nil
}

/*
apply writes the change in the transaction.
*/
func (a *UpdateWidgetSettings) apply(
	ctx context.Context, tx *database.Tx, in UpdateWidgetSettingsInput,
) (models.WidgetSetting, error) {
	q, ws, now := models.New(tx), in.Membership.Workspace.ID, a.Clock.Now()

	before, err := q.FindWidgetSettings(ctx, models.FindWidgetSettingsParams{WorkspaceID: ws, ID: in.WidgetID})
	if errors.Is(err, database.ErrNotFound) {
		return before, exceptions.ErrNotFound
	}

	if err != nil {
		return before, fmt.Errorf("find widget: %w", err)
	}

	moved, err := a.moveInbox(ctx, q, before, in.Change.InboxID)
	if err != nil {
		return before, err
	}

	params, err := changedParams(before, in.Change)
	if err != nil {
		return before, err
	}

	params.Now, params.WorkspaceID, params.ID = now, ws, before.ID

	changes := changedFields(before, params)
	if moved {
		changes = append(changes, "inbox_id")
	}

	if len(changes) == 0 {
		return before, nil
	}

	after, err := q.SaveWidgetSettings(ctx, params)
	if err != nil {
		return after, fmt.Errorf("save widget: %w", err)
	}

	return after, a.Events.Dispatch(ctx, tx, appevents.WidgetSettingsUpdated{
		WorkspaceID: ws, WidgetID: after.ID, ActorID: in.Membership.Member.ID, Name: after.Name, Changes: changes,
		Sensitive: sensitive(before, after, changes),
	})
}

/*
moveInbox sends the widget's new conversations to another inbox, and reports
whether it did. Conversations already started stay where they are.
*/
func (a *UpdateWidgetSettings) moveInbox(
	ctx context.Context, q *models.Queries, widget models.WidgetSetting, id *ids.UUID,
) (bool, error) {
	if id == nil {
		return false, nil
	}

	inbox, err := findInbox(ctx, q, widget.WorkspaceID, id)
	if err != nil {
		return false, err
	}

	conn, err := q.FindChannelConnection(ctx, models.FindChannelConnectionParams{
		WorkspaceID: widget.WorkspaceID, ID: widget.ChannelConnectionID,
	})
	if err != nil {
		return false, fmt.Errorf("find widget channel: %w", err)
	}

	if conn.InboxID == inbox.ID {
		return false, nil
	}

	err = q.MoveChannelConnectionToInbox(ctx, models.MoveChannelConnectionToInboxParams{
		InboxID: inbox.ID, Now: a.Clock.Now(), WorkspaceID: widget.WorkspaceID, ID: conn.ID,
	})
	if err != nil {
		return false, fmt.Errorf("move widget channel: %w", err)
	}

	return true, nil
}

/*
set copies a sent value over the current one.
*/
func set[T any](current *T, sent *T) {
	if sent != nil {
		*current = *sent
	}
}

/*
changedParams returns the widget's values with the change applied.
*/
func changedParams(w models.WidgetSetting, c SettingsChange) (models.SaveWidgetSettingsParams, error) {
	p := models.SaveWidgetSettingsParams{
		Name: w.Name, Enabled: w.Enabled, Appearance: w.Appearance, Texts: w.Texts, Locale: w.Locale,
		ReplyTime: w.ReplyTime, ShowTeamAvatars: w.ShowTeamAvatars, ShowAvailability: w.ShowAvailability,
		AllowedOrigins: w.AllowedOrigins, AllowAnonymous: w.AllowAnonymous, CollectEmail: w.CollectEmail,
		RequireEmailWhenOffline: w.RequireEmailWhenOffline, AttachmentsEnabled: w.AttachmentsEnabled,
		MaxAttachmentMb: w.MaxAttachmentMb, HelpCenterID: w.HelpCenterID, HelpTabEnabled: w.HelpTabEnabled,
		ChangelogTabEnabled: w.ChangelogTabEnabled, IntercomShim: w.IntercomShim,
		NativeAppsEnabled: w.NativeAppsEnabled, ShowBranding: w.ShowBranding,
	}

	applySwitches(&p, c)

	if err := applyChecked(&p, c); err != nil {
		return p, err
	}

	if c.HelpCenterSent {
		p.HelpCenterID = c.HelpCenterID
	}

	if c.ShowBrandingSent {
		p.ShowBranding = c.ShowBranding
	}

	return p, checkTabs(p)
}

/*
applySwitches copies the fields that need no checking.
*/
func applySwitches(p *models.SaveWidgetSettingsParams, c SettingsChange) {
	set(&p.Enabled, c.Enabled)
	set(&p.ShowTeamAvatars, c.ShowTeamAvatars)
	set(&p.ShowAvailability, c.ShowAvailability)
	set(&p.AllowAnonymous, c.AllowAnonymous)
	set(&p.RequireEmailWhenOffline, c.RequireEmailWhenOffline)
	set(&p.AttachmentsEnabled, c.AttachmentsEnabled)
	set(&p.HelpTabEnabled, c.HelpTabEnabled)
	set(&p.ChangelogTabEnabled, c.ChangelogTabEnabled)
	set(&p.IntercomShim, c.IntercomShim)
	set(&p.NativeAppsEnabled, c.NativeAppsEnabled)
}

/*
applyChecked copies the fields with rules of their own.
*/
func applyChecked(p *models.SaveWidgetSettingsParams, c SettingsChange) error {
	return errors.Join(
		applyName(p, c.Name), applyOptions(p, c), applyOrigins(p, c), applyStored(p, c),
	)
}

/*
applyName cleans and checks the name.
*/
func applyName(p *models.SaveWidgetSettingsParams, name *string) error {
	if name == nil {
		return nil
	}

	cleaned := str.Clean(*name)
	if cleaned == "" || len([]rune(cleaned)) > maxWidgetName {
		return exceptions.WidgetField("name", "Use 1 to 60 characters.")
	}

	p.Name = cleaned

	return nil
}

/*
applyOptions checks the fields that take one of a list.
*/
func applyOptions(p *models.SaveWidgetSettingsParams, c SettingsChange) error {
	if c.ReplyTime != nil {
		if problem := oneOf("auto", "minutes", "hours", "day")(*c.ReplyTime); problem != "" {
			return exceptions.WidgetField("reply_time", problem)
		}

		p.ReplyTime = *c.ReplyTime
	}

	if c.CollectEmail != nil {
		if problem := oneOf("never", "offline", "always")(*c.CollectEmail); problem != "" {
			return exceptions.WidgetField("collect_email", problem)
		}

		p.CollectEmail = *c.CollectEmail
	}

	if c.Locale != nil {
		locale, ok := NormalizeWidgetLocale(*c.Locale)
		if !ok {
			return exceptions.WidgetField("locale", "Use auto or a language tag such as fr.")
		}

		p.Locale = locale
	}

	if c.MaxAttachmentMB != nil {
		if *c.MaxAttachmentMB < 1 || *c.MaxAttachmentMB > 25 {
			return exceptions.WidgetField("max_attachment_mb", "Use a whole number from 1 to 25.")
		}

		p.MaxAttachmentMb = *c.MaxAttachmentMB
	}

	return nil
}

/*
applyOrigins normalizes the allowed sites.
*/
func applyOrigins(p *models.SaveWidgetSettingsParams, c SettingsChange) error {
	if !c.AllowedOriginsSent {
		return nil
	}

	origins, err := NormalizeOrigins(c.AllowedOrigins)
	if err != nil {
		return err
	}

	p.AllowedOrigins = origins

	return nil
}

/*
applyStored merges the appearance and texts.
*/
func applyStored(p *models.SaveWidgetSettingsParams, c SettingsChange) error {
	if c.Appearance != nil {
		merged, err := mergeAppearance(p.Appearance, c.Appearance)
		if err != nil {
			return err
		}

		p.Appearance = merged
	}

	if c.Texts != nil {
		merged, err := mergeTexts(p.Texts, c.Texts)
		if err != nil {
			return err
		}

		p.Texts = merged
	}

	return nil
}

/*
checkTabs refuses a tab with nothing behind it.
*/
func checkTabs(p models.SaveWidgetSettingsParams) error {
	if p.HelpCenterID != nil {
		return nil
	}

	if p.HelpTabEnabled {
		return exceptions.WidgetField("help_tab_enabled", "Link a help center first.")
	}

	if p.ChangelogTabEnabled {
		return exceptions.WidgetField("changelog_tab_enabled", "Link a help center first.")
	}

	return nil
}

/*
sensitiveFields are the fields that decide who may use a widget.
*/
func sensitiveFields() []string {
	return []string{"allowed_origins", "allow_anonymous", "native_apps_enabled"}
}

/*
sensitive returns the from and to of each changed field that decides who may
use the widget.
*/
func sensitive(before, after models.WidgetSetting, changes []string) map[string]any {
	values := func(w models.WidgetSetting) map[string]any {
		return map[string]any{
			"allowed_origins": w.AllowedOrigins, "allow_anonymous": w.AllowAnonymous,
			"native_apps_enabled": w.NativeAppsEnabled,
		}
	}
	from, to, out := values(before), values(after), map[string]any{}

	for _, field := range sensitiveFields() {
		if slices.Contains(changes, field) {
			out[field] = map[string]any{"from": from[field], "to": to[field]}
		}
	}

	return out
}

/*
changedFields names the fields whose values the change moves, by comparing
both sides' JSON, the form every field is stored and answered in.
*/
func changedFields(before models.WidgetSetting, p models.SaveWidgetSettingsParams) []string {
	after := before
	after.Name, after.Enabled, after.Appearance, after.Texts = p.Name, p.Enabled, p.Appearance, p.Texts
	after.Locale, after.ReplyTime = p.Locale, p.ReplyTime
	after.ShowTeamAvatars, after.ShowAvailability = p.ShowTeamAvatars, p.ShowAvailability
	after.AllowedOrigins, after.AllowAnonymous = p.AllowedOrigins, p.AllowAnonymous
	after.CollectEmail, after.RequireEmailWhenOffline = p.CollectEmail, p.RequireEmailWhenOffline
	after.AttachmentsEnabled, after.MaxAttachmentMb = p.AttachmentsEnabled, p.MaxAttachmentMb
	after.HelpCenterID, after.HelpTabEnabled = p.HelpCenterID, p.HelpTabEnabled
	after.ChangelogTabEnabled, after.IntercomShim = p.ChangelogTabEnabled, p.IntercomShim
	after.NativeAppsEnabled, after.ShowBranding = p.NativeAppsEnabled, p.ShowBranding

	left, right := asFields(before), asFields(after)
	changes := []string{}

	for field, value := range right {
		if !jsonEqual(left[field], value) {
			changes = append(changes, field)
		}
	}

	slices.Sort(changes)

	return changes
}

/*
asFields returns the widget's fields by name.
*/
func asFields(w models.WidgetSetting) map[string]json.RawMessage {
	encoded, _ := json.Marshal(w)
	out := map[string]json.RawMessage{}
	_ = json.Unmarshal(encoded, &out)

	return out
}

/*
jsonEqual compares two JSON values by meaning, so an object's key order does
not count as a change.
*/
func jsonEqual(a, b json.RawMessage) bool {
	var left, right any

	_ = json.Unmarshal(a, &left)
	_ = json.Unmarshal(b, &right)

	first, _ := json.Marshal(left)
	second, _ := json.Marshal(right)

	return string(first) == string(second)
}
