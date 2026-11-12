/*
|--------------------------------------------------------------------------
| Update Workspace
|--------------------------------------------------------------------------
|
| Admins rename a workspace, move it to a new address, and set the
| language and time zone members and customers fall back to. A renamed
| workspace renames its organization too while that organization holds
| no other workspace, so the two never drift apart.
|
| A new address keeps the old one held for thirty days: links to it keep
| reaching this workspace and nobody else may take it, while this
| workspace may take it back. Whether a workspace is a test workspace is
| decided when it is created and never changes.
|
| Requiring a second factor from every member locks people out, so only
| the person themselves, by browser or app, may switch it, having just
| proven who they are and holding a second factor of their own. Turning
| it on mails every member who has none, since from that moment they
| cannot open the workspace until they add one.
|
*/

package workspaces

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	fauth "github.com/dimmihq/dimmi/framework/auth"
	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/support/str"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/auth/authz"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
)

/*
fieldRequireTwoFactor names the requirement of a second factor among
the fields an update changed.
*/
const fieldRequireTwoFactor = "require_two_factor"

/*
fieldCustomerFacingName names the name customers see among the fields
an update changed.
*/
const fieldCustomerFacingName = "customer_facing_name"

/*
UpdateWorkspaceInput is who changes the workspace, as the caller and
through their membership of it, and the fields to change; nil leaves a
field as it is. CustomerFacingName sets the name customers see, and
ClearCustomerFacingName removes it so they see the workspace's name.
*/
type UpdateWorkspaceInput struct {
	Principal        *appauth.Principal
	Membership       authz.Membership
	Name             *string
	Slug             *string
	Timezone         *string
	Locale           *string
	TestMode         *bool
	RequireTwoFactor *bool
	ShowViewers      *bool
	UndoSendSeconds  *int
	SeenReceipts     *bool
	SeenReceiptName  *bool
	GIFPicker        *bool
	TeamMentions     *bool

	CustomerFacingName      *string
	ClearCustomerFacingName bool
	LinkSafety              *LinkSafetyChange
	Retention               *RetentionChange
}

/*
UpdateWorkspace changes the workspace's settings.

Rules:
  - The caller needs settings:write.
  - Requiring a second factor is set only by the user themselves, by
    browser or app, confirmed within ConfirmWindow and with a second
    factor of their own.
  - Test mode cannot change.
  - A new slug must not be reserved, used by another workspace, or held
    by one that gave it up within thirty days; the old slug is held for
    this workspace.
  - A rename renames the organization while it has no other workspace.
  - The name customers see is set or removed in the settings; removed,
    customers see the workspace's name.
  - Values equal to the current ones change nothing.
  - Turning the requirement on mails every active member without a
    second factor.

Events: [appevents.WorkspaceUpdated] when anything changed.

Errors: [exceptions.ErrForbidden], [exceptions.ErrFirstPartyOnly],
[fauth.ErrConfirmationRequired], [exceptions.ErrTwoFactorNotEnabled],
[exceptions.ErrTestModeImmutable],
[exceptions.ErrSlugReserved], [exceptions.ErrSlugTaken], or a wrapped
database error.
*/
type UpdateWorkspace struct {
	DB            *database.DB
	Events        *events.Dispatcher
	Clock         clock.Clock
	Mail          *mail.Manager
	Links         appauth.Links
	ConfirmWindow time.Duration
}

/*
Handle applies the changes and returns the workspace as it now is.
*/
func (a *UpdateWorkspace) Handle(ctx context.Context, in UpdateWorkspaceInput) (models.Workspace, error) {
	if err := a.authorize(in); err != nil {
		return models.Workspace{}, err
	}

	var updated models.Workspace

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		current, err := q.LockWorkspace(ctx, in.Membership.Workspace.ID)
		if err != nil {
			return fmt.Errorf("lock workspace: %w", err)
		}

		if in.TestMode != nil && *in.TestMode != current.TestMode {
			return exceptions.ErrTestModeImmutable
		}

		params, changes, err := a.changes(ctx, q, current, in)
		if err != nil || len(changes) == 0 {
			updated = current

			return err
		}

		updated, err = q.UpdateWorkspace(ctx, params)
		if err != nil {
			return fmt.Errorf("update workspace: %w", err)
		}

		return a.finish(ctx, tx, updated, appevents.WorkspaceUpdated{
			WorkspaceID: updated.ID, UpdatedBy: in.Membership.Member.ID, Changes: changes,
		})
	})

	return updated, err
}

/*
authorize checks that the caller may make the changes.
*/
func (a *UpdateWorkspace) authorize(in UpdateWorkspaceInput) error {
	if !in.Membership.Can("settings:write") {
		return exceptions.ErrForbidden
	}

	if in.RequireTwoFactor == nil {
		return nil
	}

	if in.Principal == nil || !in.Principal.IsFirstParty() {
		return exceptions.ErrFirstPartyOnly
	}

	if !fauth.Confirmed(in.Principal, a.Clock.Now(), a.ConfirmWindow) {
		return fauth.ErrConfirmationRequired
	}

	if !in.Principal.User.TwoFactorEnabled() {
		return exceptions.ErrTwoFactorNotEnabled
	}

	return nil
}

/*
changes returns the stored values after the update and the names of the
fields that change, sorted.
*/
func (a *UpdateWorkspace) changes(
	ctx context.Context, q *models.Queries, w models.Workspace, in UpdateWorkspaceInput,
) (models.UpdateWorkspaceParams, []string, error) {
	now := a.Clock.Now()
	params := models.UpdateWorkspaceParams{
		ID: w.ID, Name: w.Name, Slug: w.Slug, Timezone: w.Timezone, Locale: w.Locale, Settings: w.Settings, Now: now,
	}

	var changes []string

	change := func(field string, value, target *string) {
		if value != nil && *value != *target {
			*target = *value
			changes = append(changes, field)
		}
	}

	change("name", cleaned(in.Name), &params.Name)
	change("timezone", in.Timezone, &params.Timezone)
	change("locale", in.Locale, &params.Locale)

	if in.Slug != nil && *in.Slug != w.Slug {
		settings, err := moveSlug(ctx, q, w, models.CountWorkspacesWithSlugParams{
			WorkspaceID: w.ID, Slug: *in.Slug, Now: now,
		})
		if err != nil {
			return params, nil, err
		}

		params.Slug, params.Settings = *in.Slug, settings
		changes = append(changes, "slug")
	}

	settingChanges, err := settingsChange(&params, w, in)
	if err != nil {
		return params, nil, err
	}

	changes = append(changes, settingChanges...)

	slices.Sort(changes)

	return params, changes, nil
}

/*
inboxFlag is an on or off inbox setting of the workspace: its key, the
value asked for, nil to leave it, and the value it has.
*/
type inboxFlag struct {
	key  string
	want *bool
	has  bool
}

/*
settingsChange applies to params the changes kept in the settings
document: the requirement of a second factor, whether inbox lists show
viewers, whether seen receipts name the teammate, the name customers
see and the link checks, and returns the names of those that changed.
*/
func settingsChange(
	params *models.UpdateWorkspaceParams, w models.Workspace, in UpdateWorkspaceInput,
) ([]string, error) {
	var changes []string

	if in.RequireTwoFactor != nil && *in.RequireTwoFactor != w.RequiresTwoFactor() {
		settings, err := models.WithTwoFactorRequired(params.Settings, *in.RequireTwoFactor)
		if err != nil {
			return nil, err
		}

		params.Settings = settings
		changes = append(changes, fieldRequireTwoFactor)
	}

	inbox, err := inboxSettingChanges(params, w, in)

	return append(changes, inbox...), err
}

/*
inboxSettingChanges writes the inbox settings the input changes into the
params, returning their names.
*/
func inboxSettingChanges(
	params *models.UpdateWorkspaceParams, w models.Workspace, in UpdateWorkspaceInput,
) ([]string, error) {
	var changes []string

	for _, flag := range []inboxFlag{
		{key: "show_viewers_on_list", want: in.ShowViewers, has: w.ShowsViewersOnList()},
		{key: "seen_receipts", want: in.SeenReceipts, has: w.ShowsSeenReceipts()},
		{key: "seen_receipt_shows_name", want: in.SeenReceiptName, has: w.SeenReceiptShowsName()},
		{key: "gif_picker", want: in.GIFPicker, has: w.GIFPickerOn()},
		{key: "team_mentions_enabled", want: in.TeamMentions, has: w.TeamMentionsOn()},
	} {
		if flag.want == nil || *flag.want == flag.has {
			continue
		}

		settings, err := models.WithSetting(params.Settings, "inbox", flag.key, *flag.want)
		if err != nil {
			return nil, err
		}

		params.Settings = settings
		changes = append(changes, flag.key)
	}

	if in.UndoSendSeconds != nil && *in.UndoSendSeconds != w.UndoSendSeconds() {
		settings, err := models.WithSetting(params.Settings, "inbox", "undo_send_seconds", *in.UndoSendSeconds)
		if err != nil {
			return nil, err
		}

		params.Settings = settings
		changes = append(changes, "undo_send_seconds")
	}

	settings, changed, err := customerFacingName(params.Settings, w, in)
	if err != nil {
		return nil, err
	}

	if changed {
		params.Settings = settings
		changes = append(changes, fieldCustomerFacingName)
	}

	return safetyChanges(params, w, in, changes)
}

/*
safetyChanges applies the link checks and retention periods, adding
their names to the changes when they changed.
*/
func safetyChanges(
	params *models.UpdateWorkspaceParams, w models.Workspace, in UpdateWorkspaceInput, changes []string,
) ([]string, error) {
	changed, err := applyLinkSafety(params, w, in.LinkSafety)
	if err != nil {
		return nil, err
	}

	if changed {
		changes = append(changes, "link_safety")
	}

	changed, err = applyRetention(params, w, in.Retention, params.Now)
	if err != nil {
		return nil, err
	}

	if changed {
		changes = append(changes, "retention")
	}

	return changes, nil
}

/*
customerFacingName returns the settings with the name customers see as
asked, and whether it changed.
*/
func customerFacingName(
	settings json.RawMessage, w models.Workspace, in UpdateWorkspaceInput,
) (json.RawMessage, bool, error) {
	next := cleaned(in.CustomerFacingName)
	if next == nil && !in.ClearCustomerFacingName {
		return settings, false, nil
	}

	if current := w.CustomerFacingNameSetting(); ptrEqual(current, next) {
		return settings, false, nil
	}

	out, err := models.WithCustomerFacingName(settings, next)

	return out, err == nil, err
}

/*
ptrEqual reports whether two optional names are the same, both absent
counting as the same.
*/
func ptrEqual(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}

	return *a == *b
}

/*
finish renames the organization when the name changed, tells members
without a second factor when one became required, and announces the
update.
*/
func (a *UpdateWorkspace) finish(
	ctx context.Context, tx *database.Tx, w models.Workspace, updated appevents.WorkspaceUpdated,
) error {
	if slices.Contains(updated.Changes, fieldRequireTwoFactor) && w.RequiresTwoFactor() {
		if err := a.mailUnprotected(ctx, tx, w); err != nil {
			return err
		}
	}

	if slices.Contains(updated.Changes, "name") {
		err := models.New(tx).RenameOrganization(ctx, models.RenameOrganizationParams{
			ID: w.OrganizationID, Name: w.Name, Now: w.UpdatedAt,
		})
		if err != nil {
			return fmt.Errorf("rename organization: %w", err)
		}
	}

	return a.Events.Dispatch(ctx, tx, updated)
}

/*
mailUnprotected queues the mail that tells every active member without
a second factor the workspace now requires one.
*/
func (a *UpdateWorkspace) mailUnprotected(ctx context.Context, tx *database.Tx, w models.Workspace) error {
	users, err := models.New(tx).ListMembersWithoutTwoFactor(ctx, w.ID)
	if err != nil {
		return fmt.Errorf("list members without two-factor: %w", err)
	}

	for _, row := range users {
		err := appmail.ToUser(a.Mail, row.User).QueueTx(ctx, tx, appmail.TwoFactorRequiredMail{
			Name: row.User.Name, Workspace: w.Name, SecurityURL: a.Links.Security(),
		})
		if err != nil {
			return fmt.Errorf("queue two-factor required mail: %w", err)
		}
	}

	return nil
}

/*
cleaned returns the name without invisible characters and outer
spaces, nil for none.
*/
func cleaned(name *string) *string {
	if name == nil {
		return nil
	}

	clean := str.Clean(*name)

	return &clean
}

/*
moveSlug checks that the workspace may take the slug the check names
and returns its settings with the old slug held for it.
*/
func moveSlug(
	ctx context.Context, q *models.Queries, w models.Workspace, check models.CountWorkspacesWithSlugParams,
) (json.RawMessage, error) {
	if reservedSlug(check.Slug) {
		return nil, exceptions.ErrSlugReserved
	}

	taken, err := slugTaken(ctx, q, check)
	if err != nil {
		return nil, err
	}

	if taken {
		return nil, takenBy(ctx, q, check.Slug)
	}

	return holdSlug(w.Settings, w.Slug, check.Slug, check.Now)
}

/*
takenBy returns why a taken slug cannot be had: another workspace uses
it, or one gave it up recently and still holds it.
*/
func takenBy(ctx context.Context, q *models.Queries, slug string) error {
	_, err := q.FindWorkspaceBySlug(ctx, slug)
	if errors.Is(err, database.ErrNotFound) {
		return exceptions.ErrSlugReserved
	}

	if err != nil {
		return fmt.Errorf("find workspace by slug: %w", err)
	}

	return exceptions.ErrSlugTaken
}
