/*
|--------------------------------------------------------------------------
| Manage Companies
|--------------------------------------------------------------------------
|
| Teammates read companies with contacts:read and edit them with
| contacts:write, since a company is part of knowing the customer.
| Deleting one takes its own permission: it is soft, and its people stay
| as contacts without a company.
|
*/

package companies

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/tags"
	"github.com/dimmihq/dimmi/app/auth/authz"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/attributes"
)

/*
externalIDConstraint keeps a workspace's company external ids apart.
*/
const externalIDConstraint = "companies_workspace_id_external_id_key"

/*
CreateInput is who adds which company.
*/
type CreateInput struct {
	Membership authz.Membership
	Fields     Fields
}

/*
CreateCompany adds a company to the workspace.

Rules:
  - The caller needs contacts:write.
  - An external id another company holds is refused.

Events: [appevents.CompanyCreated].

Errors: [exceptions.ErrForbidden], [exceptions.ErrCompanyExists],
[exceptions.ErrInvalidAttributes], or a wrapped database error.
*/
type CreateCompany struct {
	DB     *database.DB
	Events *events.Dispatcher
	Clock  clock.Clock
}

/*
Handle adds the company and returns it.
*/
func (a *CreateCompany) Handle(ctx context.Context, in CreateInput) (models.Company, error) {
	if !in.Membership.Can("contacts:write") {
		return models.Company{}, exceptions.ErrForbidden
	}

	var out models.Company

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		ws := in.Membership.Workspace.ID

		var err error

		out, err = create(ctx, models.New(tx), Identity{WorkspaceID: ws, Fields: in.Fields, Now: a.Clock.Now()})
		if err != nil {
			return err
		}

		return a.Events.Dispatch(ctx, tx, appevents.CompanyCreated{
			WorkspaceID: ws, CompanyID: out.ID, ActorID: in.Membership.Member.ID,
		})
	})

	return out, err
}

/*
UpdateInput is who changes which company and how. Fields holds the
details sent; Cleared names those sent as null.
*/
type UpdateInput struct {
	Membership authz.Membership
	CompanyID  ids.UUID
	Fields     Fields
	Cleared    map[string]bool
	Notes      *string
	AddTags    tags.Refs
	RemoveTags []ids.UUID
}

/*
UpdateCompany changes a company.

Rules:
  - The caller needs contacts:write.
  - A company holds at most 50 tags.
  - Values equal to the current ones change nothing.

Events: [appevents.CompanyUpdated] when anything changed.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound],
[exceptions.ErrCompanyExists], [exceptions.ErrInvalidAttributes],
[exceptions.ErrTagLimitReached], or a wrapped database error.
*/
type UpdateCompany struct {
	DB     *database.DB
	Events *events.Dispatcher
	Clock  clock.Clock
}

/*
Handle applies the change and returns the company as it now is.
*/
func (a *UpdateCompany) Handle(ctx context.Context, in UpdateInput) (Detail, error) {
	if !in.Membership.Can("contacts:write") {
		return Detail{}, exceptions.ErrForbidden
	}

	var out Detail

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q, ws := models.New(tx), in.Membership.Workspace.ID

		current, err := q.LockCompany(ctx, models.LockCompanyParams{WorkspaceID: ws, ID: in.CompanyID})
		if errors.Is(err, database.ErrNotFound) {
			return exceptions.ErrNotFound
		}

		if err != nil {
			return fmt.Errorf("lock company: %w", err)
		}

		u := companyUpdate{in: in, now: a.Clock.Now(), before: current}

		company, err := u.write(ctx, q)
		if err != nil {
			return err
		}

		if out, err = LoadDetail(ctx, q, company); err != nil || len(u.changes) == 0 {
			return err
		}

		return a.Events.Dispatch(ctx, tx, appevents.CompanyUpdated{
			WorkspaceID: ws, CompanyID: company.ID, ActorID: in.Membership.Member.ID, Changes: u.changes,
			ChangedAttributeKeys: u.keys,
		})
	})

	return out, err
}

/*
companyUpdate works out and writes one change to a company.
*/
type companyUpdate struct {
	in      UpdateInput
	now     time.Time
	before  models.Company
	params  models.UpdateCompanyParams
	changes []string
	keys    []string
}

/*
paramsOf returns the company's stored values as the parameters of an
update.
*/
func paramsOf(c models.Company, now time.Time) models.UpdateCompanyParams {
	return models.UpdateCompanyParams{
		ExternalID: c.ExternalID, Name: c.Name, Domain: c.Domain, CustomAttributes: c.CustomAttributes, Notes: c.Notes,
		Now: now, WorkspaceID: c.WorkspaceID, ID: c.ID,
	}
}

/*
write applies each field and writes the company when something changed.
*/
func (u *companyUpdate) write(ctx context.Context, q *models.Queries) (models.Company, error) {
	b, f := u.before, u.in.Fields
	u.params = paramsOf(b, u.now)

	if f.Name != nil && strings.TrimSpace(*f.Name) != "" && *f.Name != b.Name {
		u.params.Name = *f.Name
		u.changes = append(u.changes, "name")
	}

	u.text("external_id", &u.params.ExternalID, f.ExternalID)
	u.text("domain", &u.params.Domain, NormalizeDomain(f.Domain))
	u.text("notes", &u.params.Notes, u.in.Notes)

	merged, keys, err := attributes.Merge(b.CustomAttributes, f.CustomAttributes)
	if err != nil {
		return b, err
	}

	if len(keys) > 0 {
		u.params.CustomAttributes, u.keys = merged, keys
		u.changes = append(u.changes, "custom_attributes")
	}

	if err := u.tags(ctx, q); err != nil {
		return b, err
	}

	if len(u.changes) == 0 {
		return b, nil
	}

	company, err := q.UpdateCompany(ctx, u.params)
	if database.IsUniqueViolation(err, externalIDConstraint) {
		return b, exceptions.ErrCompanyExists
	}

	if err != nil {
		return b, fmt.Errorf("update company: %w", err)
	}

	return company, nil
}

/*
text applies one optional text field: the value sent, or nothing when
the request cleared it.
*/
func (u *companyUpdate) text(field string, current **string, value *string) {
	next := *current
	if value != nil {
		next = value
	}

	if u.in.Cleared[field] {
		next = nil
	}

	same := (*current == nil && next == nil) || (*current != nil && next != nil && **current == *next)
	if same {
		return
	}

	*current = next
	u.changes = append(u.changes, field)
}

/*
tags adds and removes the tags named, up to the limit on a company.
*/
func (u *companyUpdate) tags(ctx context.Context, q *models.Queries) error {
	ws, id := u.before.WorkspaceID, u.before.ID

	var added []ids.UUID

	if !u.in.AddTags.Empty() {
		tagIDs, err := tags.Resolve(ctx, q, tags.Scope{WorkspaceID: ws, Now: u.now}, u.in.AddTags)
		if err != nil {
			return err
		}

		added, err = q.TagCompany(ctx, models.TagCompanyParams{
			WorkspaceID: ws, CompanyID: id, TagIds: tagIDs, CreatedByType: "member",
			CreatedByID: &u.in.Membership.Member.ID, Now: u.now,
		})
		if err != nil {
			return fmt.Errorf("tag company: %w", err)
		}
	}

	removed, err := q.UntagCompany(ctx, models.UntagCompanyParams{
		WorkspaceID: ws, CompanyID: id, TagIds: u.in.RemoveTags,
	})
	if err != nil {
		return fmt.Errorf("untag company: %w", err)
	}

	if len(added) == 0 && len(removed) == 0 {
		return nil
	}

	count, err := q.CountCompanyTags(ctx, models.CountCompanyTagsParams{WorkspaceID: ws, CompanyID: id})
	if err != nil {
		return fmt.Errorf("count company tags: %w", err)
	}

	if count > tags.MaxOnRecord {
		return exceptions.ErrTagLimitReached
	}

	u.changes = append(u.changes, "tags")

	return nil
}

/*
DeleteCompany moves a company to the trash, its people left without a
company.

Rules:
  - The caller needs companies:delete.

Events: [appevents.CompanyDeleted].

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], or a
wrapped database error.
*/
type DeleteCompany struct {
	DB     *database.DB
	Events *events.Dispatcher
	Clock  clock.Clock
}

/*
Handle deletes the company.
*/
func (a *DeleteCompany) Handle(ctx context.Context, in ShowInput) error {
	if !in.Membership.Can("companies:delete") {
		return exceptions.ErrForbidden
	}

	return a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q, ws, now := models.New(tx), in.Membership.Workspace.ID, a.Clock.Now()

		rows, err := q.DeleteCompany(ctx, models.DeleteCompanyParams{Now: now, WorkspaceID: ws, ID: in.CompanyID})
		if err != nil {
			return fmt.Errorf("delete company: %w", err)
		}

		if rows == 0 {
			return exceptions.ErrNotFound
		}

		_, err = q.DetachCompanyContacts(ctx, models.DetachCompanyContactsParams{
			Now: now, WorkspaceID: ws, CompanyID: in.CompanyID,
		})
		if err != nil {
			return fmt.Errorf("detach contacts: %w", err)
		}

		return a.Events.Dispatch(ctx, tx, appevents.CompanyDeleted{
			WorkspaceID: ws, CompanyID: in.CompanyID, ActorID: in.Membership.Member.ID,
		})
	})
}
