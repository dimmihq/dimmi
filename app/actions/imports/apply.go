/*
|--------------------------------------------------------------------------
| Applying Rows
|--------------------------------------------------------------------------
|
| A customer row goes through the same upsert the API uses, so it is
| matched by external id and then by a proven address exactly as a
| developer's server would be, and its events follow as usual. A company
| row is found by external id, then domain. A row that matches someone
| already there changes them, or is skipped when the import leaves
| existing records alone. Tags named in a row are made when new and
| added, never taken away. Each row is its own transaction, so one bad
| row never stops the others.
|
*/

package imports

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/companies"
	"github.com/dimmihq/dimmi/app/actions/contacts"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
)

/*
The outcomes of a row.
*/
const (
	OutcomeCreated = "created"
	OutcomeUpdated = "updated"
	OutcomeSkipped = "skipped"
	OutcomeFailed  = "failed"
)

/*
applier writes rows for one member, keeping what each row changed
under the import when it runs for real.
*/
type applier struct {
	db       *database.DB
	upsert   *contacts.UpsertIdentifiedContact
	m        authz.Membership
	kind     string
	mode     string
	now      func() time.Time
	importID ids.UUID
}

/*
predict says what a row would come to, without writing anything.
*/
func (a applier) predict(ctx context.Context, q *models.Queries, r record) (string, error) {
	found, err := a.matches(ctx, q, r)
	if err != nil || !found {
		return OutcomeCreated, err
	}

	if a.mode == ModeSkip {
		return OutcomeSkipped, nil
	}

	return OutcomeUpdated, nil
}

/*
matches reports whether the row names someone already there.
*/
func (a applier) matches(ctx context.Context, q *models.Queries, r record) (bool, error) {
	ws := a.m.Workspace.ID

	if a.kind == KindCompanies {
		return companyMatches(ctx, q, ws, r.self)
	}

	if r.contact.ExternalID != nil {
		_, err := q.FindContactByExternalID(ctx, models.FindContactByExternalIDParams{
			WorkspaceID: ws, ExternalID: r.contact.ExternalID,
		})
		if found, err := present(err); found || err != nil {
			return found, err
		}
	}

	email := contacts.NormalizeEmail(r.contact.Email)
	if email == nil {
		return false, nil
	}

	_, err := q.FindUpsertCandidateByEmail(ctx, models.FindUpsertCandidateByEmailParams{WorkspaceID: ws, Email: email})

	return present(err)
}

/*
companyMatches reports whether a live company has the external id or
the domain.
*/
func companyMatches(ctx context.Context, q *models.Queries, ws ids.UUID, f companies.Fields) (bool, error) {
	if f.ExternalID != nil {
		_, err := q.FindCompanyByExternalID(ctx, models.FindCompanyByExternalIDParams{
			WorkspaceID: ws, ExternalID: f.ExternalID,
		})
		if found, err := present(err); found || err != nil {
			return found, err
		}
	}

	domain := companies.NormalizeDomain(f.Domain)
	if domain == nil {
		return false, nil
	}

	_, err := q.FindCompanyByDomain(ctx, models.FindCompanyByDomainParams{WorkspaceID: ws, Domain: domain})

	return present(err)
}

/*
present turns a lookup's error into whether it found a row.
*/
func present(err error) (bool, error) {
	if errors.Is(err, database.ErrNotFound) {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("look up record: %w", err)
	}

	return true, nil
}

/*
apply writes the row and returns its outcome.
*/
func (a applier) apply(ctx context.Context, r record) (string, error) {
	if a.mode == ModeSkip {
		found, err := a.matches(ctx, reads(ctx, a.db), r)
		if err != nil || found {
			return OutcomeSkipped, err
		}
	}

	if a.kind == KindCompanies {
		return a.company(ctx, r)
	}

	return a.contact(ctx, r)
}

/*
contact upserts the customer, puts them in the company the row names
and adds its tags.
*/
func (a applier) contact(ctx context.Context, r record) (string, error) {
	var employer *companies.Fields
	if r.company != nil && (r.company.ExternalID != nil || companies.NormalizeDomain(r.company.Domain) != nil) {
		employer = r.company
	}

	before, err := a.priorContact(ctx, r)
	if err != nil {
		return OutcomeFailed, err
	}

	up, err := a.upsert.Handle(ctx, contacts.UpsertInput{Membership: a.m, Fields: r.contact, Company: employer})
	if err != nil {
		return OutcomeFailed, err
	}

	c := change{id: up.Detail.Contact.ID, created: up.Created, before: before}
	if employer == nil && r.company != nil {
		c.companyName = r.company.Name
	}

	err = a.db.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		return a.contactExtras(ctx, models.New(tx), r, c)
	})
	if err != nil {
		return OutcomeFailed, err
	}

	if up.Created {
		return OutcomeCreated, nil
	}

	return OutcomeUpdated, nil
}

/*
contactExtras puts the customer in the company the row names only by
name, when no other key named it, adds the row's tags and keeps what
the row changed.
*/
func (a applier) contactExtras(ctx context.Context, q *models.Queries, r record, c change) error {
	if c.companyName != nil {
		if err := a.linkByName(ctx, q, c.id, *c.companyName); err != nil {
			return err
		}
	}

	tagIDs, err := a.tags(ctx, q, r.tags)
	if err != nil {
		return err
	}

	c.tags, err = q.TagContact(ctx, models.TagContactParams{
		WorkspaceID: a.m.Workspace.ID, ContactID: c.id, CreatedByType: "member", CreatedByID: &a.m.Member.ID,
		Now: a.now(), TagIds: tagIDs,
	})
	if err != nil {
		return err
	}

	return a.keepContact(ctx, q, c)
}

/*
linkByName puts the customer in the live company with the name, adding
one when there is none.
*/
func (a applier) linkByName(ctx context.Context, q *models.Queries, contact ids.UUID, name string) error {
	ws, now := a.m.Workspace.ID, a.now()

	c, err := q.FindCompanyByName(ctx, models.FindCompanyByNameParams{WorkspaceID: ws, Name: name})
	if errors.Is(err, database.ErrNotFound) {
		c, err = q.CreateCompany(ctx, models.CreateCompanyParams{
			WorkspaceID: ws, Name: name, CustomAttributes: []byte(`{}`), Now: now,
		})
	}

	if err != nil {
		return fmt.Errorf("find company by name: %w", err)
	}

	return q.LinkImportedContactCompany(ctx, models.LinkImportedContactCompanyParams{
		CompanyID: &c.ID, Now: now, WorkspaceID: ws, ID: contact,
	})
}

/*
company finds or adds the company and adds its tags.
*/
func (a applier) company(ctx context.Context, r record) (string, error) {
	outcome := OutcomeUpdated

	err := a.db.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		before, err := priorCompany(ctx, q, a.m.Workspace.ID, r)
		if err != nil {
			return err
		}

		c, created, err := companies.Identify(ctx, tx, companies.Identity{
			WorkspaceID: a.m.Workspace.ID, Fields: r.self, Now: a.now(),
		})
		if err != nil {
			return err
		}

		if created {
			outcome = OutcomeCreated
		}

		tagIDs, err := a.tags(ctx, q, r.tags)
		if err != nil {
			return err
		}

		added, err := q.TagCompany(ctx, models.TagCompanyParams{
			WorkspaceID: a.m.Workspace.ID, CompanyID: c.ID, CreatedByType: "member", CreatedByID: &a.m.Member.ID,
			Now: a.now(), TagIds: tagIDs,
		})
		if err != nil {
			return err
		}

		after, err := companySnapshot(c)
		if err != nil {
			return err
		}

		return a.keep(ctx, q, change{id: c.ID, created: created, before: before, after: after, tags: added})
	})
	if err != nil {
		return OutcomeFailed, err
	}

	return outcome, nil
}

/*
tags returns the ids of the workspace's tags with the names, making
those that are new.
*/
func (a applier) tags(ctx context.Context, q *models.Queries, names []string) ([]ids.UUID, error) {
	if len(names) == 0 {
		return nil, nil
	}

	rows, err := q.EnsureTags(ctx, models.EnsureTagsParams{Names: names, WorkspaceID: a.m.Workspace.ID, Now: a.now()})
	if err != nil {
		return nil, fmt.Errorf("ensure tags: %w", err)
	}

	out := make([]ids.UUID, 0, len(rows))
	for _, t := range rows {
		out = append(out, t.ID)
	}

	return out, nil
}

/*
reason words why a row failed for the report: the message of a refusal
a person can act on, or a plain line for anything else.
*/
func reason(err error) string {
	type messaged interface{ Message() string }

	if f, ok := errors.AsType[*fault.Error](err); ok {
		return f.Message()
	}

	var m messaged
	if errors.As(err, &m) {
		return m.Message()
	}

	return "could not be saved"
}
