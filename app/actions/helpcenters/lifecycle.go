/*
|--------------------------------------------------------------------------
| Adding and Deleting Help Centers
|--------------------------------------------------------------------------
|
| A workspace may run more than one help center, such as one per product
| or language. A new one takes a name, a language and a subdomain; left
| out, the subdomain comes from the name, with a short suffix when that
| is taken. Deleting one needs the owner's permission and the help
| center's name typed back, since its articles go with it. The default
| help center stays: the agent and new messengers read it. Messengers
| that showed a deleted help center show the default one instead.
|
*/

package helpcenters

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Failures of adding and deleting help centers.
*/
var (
	ErrDefaultHelpCenter = fault.New(fault.Conflict, "default_help_center",
		"The default help center cannot be deleted; the agent and new messengers read it.")
	ErrConfirmationMismatch = fault.New(fault.Invalid, "confirmation_mismatch",
		"Type the help center's name exactly to delete it.").OnField("confirm")
	ErrLocaleInvalid = fault.New(fault.Invalid, "locale_invalid",
		"Use a language code such as en or pt-BR.").OnField("locale")
)

/*
localePattern is a language tag such as en, fr or pt-BR.
*/
var localePattern = regexp.MustCompile(`^[a-z]{2,3}(-[A-Za-z0-9]{2,8})?$`)

/*
NewHelpCenter is a help center to add: an empty subdomain is taken from
the name, an empty locale from the workspace.
*/
type NewHelpCenter struct {
	Membership authz.Membership
	Name       string
	Subdomain  string
	Locale     string
}

/*
Create adds a help center to the workspace.
*/
func (h *HelpCenters) Create(ctx context.Context, in NewHelpCenter) (models.HelpCenter, error) {
	if !in.Membership.Can("help_centers:manage") {
		return models.HelpCenter{}, exceptions.ErrForbidden
	}
	name := strings.TrimSpace(in.Name)
	if name == "" || len([]rune(name)) > 80 {
		return models.HelpCenter{}, fault.New(fault.Invalid, "name_invalid", "Names are 1 to 80 characters.").
			OnField("name")
	}
	locale := strings.TrimSpace(in.Locale)
	if locale == "" {
		locale = localeOf(in.Membership.Workspace)
	}
	if !localePattern.MatchString(locale) {
		return models.HelpCenter{}, ErrLocaleInvalid
	}
	var out models.HelpCenter
	err := h.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		if err := h.ensure(ctx, q, in.Membership.Workspace); err != nil {
			return err
		}
		sub, err := h.pickSubdomain(ctx, q, in.Subdomain, name)
		if err != nil {
			return err
		}
		now := h.Clock.Now()
		out, err = q.CreateHelpCenter(ctx, models.CreateHelpCenterParams{
			ID: ids.NewV7At(now), WorkspaceID: in.Membership.Workspace.ID, Name: name, Subdomain: sub,
			Locale: locale, IsDefault: false, Settings: json.RawMessage(`{}`), Now: now,
		})
		if err != nil {
			return fmt.Errorf("create help center: %w", err)
		}
		return nil
	})
	return out, err
}

/*
pickSubdomain checks the subdomain asked for, or finds a free one from
the name.
*/
func (h *HelpCenters) pickSubdomain(ctx context.Context, q *models.Queries, asked, name string) (string, error) {
	if strings.TrimSpace(asked) != "" {
		return checkSubdomain(ctx, q, asked)
	}
	return h.freeSubdomain(ctx, q, name)
}

/*
Delete moves a help center that is not the default to the trash with its
articles out of readers' reach, once its name is typed back as confirm.
*/
func (h *HelpCenters) Delete(ctx context.Context, m authz.Membership, id ids.UUID, confirm string) error {
	if !m.Can("help_centers:delete") {
		return exceptions.ErrForbidden
	}
	return h.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		current, err := findHelpCenter(ctx, q, m.Workspace.ID, id)
		if err != nil {
			return err
		}
		if current.IsDefault {
			return ErrDefaultHelpCenter
		}
		if strings.TrimSpace(confirm) != current.Name {
			return ErrConfirmationMismatch
		}
		fallback, err := q.FindDefaultHelpCenter(ctx, m.Workspace.ID)
		if err != nil {
			return fmt.Errorf("find default help center: %w", err)
		}
		now := h.Clock.Now()
		forget := models.SetHelpCenterCustomDomainParams{Now: now, WorkspaceID: m.Workspace.ID, ID: id}
		if err := forgetDomain(ctx, q, forget); err != nil {
			return err
		}
		if err := q.RepointWidgetsHelpCenter(ctx, models.RepointWidgetsHelpCenterParams{
			ToHelpCenterID: &fallback.ID, Now: now, WorkspaceID: m.Workspace.ID, FromHelpCenterID: &id,
		}); err != nil {
			return fmt.Errorf("repoint messengers: %w", err)
		}
		if err := q.SoftDeleteHelpCenter(ctx, models.SoftDeleteHelpCenterParams{
			Now: now, WorkspaceID: m.Workspace.ID, ID: id,
		}); err != nil {
			return fmt.Errorf("delete help center: %w", err)
		}
		return h.trashArticles(ctx, tx, m.Workspace.ID, id)
	})
}

/*
trashArticles moves a deleted help center's articles to the trash and
out of the knowledge the agent reads.
*/
func (h *HelpCenters) trashArticles(ctx context.Context, tx *database.Tx, ws, id ids.UUID) error {
	trashed, err := models.New(tx).TrashHelpCenterArticles(ctx, models.TrashHelpCenterArticlesParams{
		Now: h.Clock.Now(), WorkspaceID: ws, HelpCenterID: id,
	})
	if err != nil {
		return fmt.Errorf("trash help center articles: %w", err)
	}
	if h.Index == nil {
		return nil
	}
	for _, article := range trashed {
		if err := h.Index.IndexArticle(ctx, tx, ws, article); err != nil {
			return err
		}
	}
	return nil
}
