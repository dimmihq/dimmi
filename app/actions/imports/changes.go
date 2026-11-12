/*
|--------------------------------------------------------------------------
| Import Changes
|--------------------------------------------------------------------------
|
| While an import runs, each record it touches leaves a change behind so
| the import can be undone: whether the row made the record, the fields
| it changed with their values before and after, and the tags it added.
| A record is read as a flat set of fields, its custom attributes under
| attribute.<key>, the same way before and after the row, so the change
| holds only what really moved. A row that changed nothing keeps
| nothing.
|
*/

package imports

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/companies"
	"github.com/dimmihq/dimmi/app/actions/contacts"
	"github.com/dimmihq/dimmi/app/models"
)

/*
snapshot is a record's importable fields, as plain JSON values.
*/
type snapshot map[string]any

/*
contactSnapshot reads the fields of a customer an import may change.
*/
func contactSnapshot(c models.Contact) (snapshot, error) {
	s := map[string]any{"email": c.Email, "name": c.Name, "phone": c.Phone, "company_id": c.CompanyID}

	return withAttributes(s, c.CustomAttributes)
}

/*
companySnapshot reads the fields of a company an import may change.
*/
func companySnapshot(c models.Company) (snapshot, error) {
	s := map[string]any{"name": c.Name, "domain": c.Domain}

	return withAttributes(s, c.CustomAttributes)
}

/*
withAttributes adds the custom attributes to the fields and turns them
all into plain JSON values, so a snapshot compares equal to one read
back from the database.
*/
func withAttributes(fields map[string]any, raw json.RawMessage) (snapshot, error) {
	attrs := map[string]any{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &attrs); err != nil {
			return nil, fmt.Errorf("read custom attributes: %w", err)
		}
	}

	for k, v := range attrs {
		fields[attributePrefix+k] = v
	}

	data, err := json.Marshal(fields)
	if err != nil {
		return nil, fmt.Errorf("encode snapshot: %w", err)
	}

	out := snapshot{}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("decode snapshot: %w", err)
	}

	return out, nil
}

/*
diff returns the fields that differ between two snapshots, with their
values on each side; a field missing on one side is null there.
*/
func diff(before, after snapshot) (snapshot, snapshot) {
	b, a := snapshot{}, snapshot{}

	keys := maps.Clone(before)
	maps.Copy(keys, after)

	for k := range keys {
		if !same(before[k], after[k]) {
			b[k], a[k] = before[k], after[k]
		}
	}

	return b, a
}

/*
same reports whether two JSON values are equal, maps in any key order.
*/
func same(a, b any) bool {
	x, errA := json.Marshal(a)
	y, errB := json.Marshal(b)

	return errA == nil && errB == nil && bytes.Equal(x, y)
}

/*
prior is the record a row matched before it ran, and how it stood.
*/
type prior struct {
	id     ids.UUID
	fields snapshot
}

/*
priorContact returns the customer the row names before it runs, matched
the way the upsert matches: by external id, following merges, then by
a proven address. Nil when the row names nobody yet.
*/
func (a applier) priorContact(ctx context.Context, r record) (*prior, error) {
	q, ws := models.New(a.db), a.m.Workspace.ID

	var (
		c   models.Contact
		err = database.ErrNotFound
	)

	if r.contact.ExternalID != nil {
		c, err = q.FindContactByExternalID(ctx, models.FindContactByExternalIDParams{
			WorkspaceID: ws, ExternalID: r.contact.ExternalID,
		})
		if err == nil {
			c, err = contacts.Survivor(ctx, q, ws, c.ID)
		}
	}

	if email := contacts.NormalizeEmail(r.contact.Email); errors.Is(err, database.ErrNotFound) && email != nil {
		c, err = q.FindUpsertCandidateByEmail(ctx, models.FindUpsertCandidateByEmailParams{
			WorkspaceID: ws, Email: email,
		})
	}

	if err != nil {
		if found, err := present(err); !found {
			return nil, err
		}
	}

	fields, err := contactSnapshot(c)
	if err != nil {
		return nil, err
	}

	return &prior{id: c.ID, fields: fields}, nil
}

/*
priorCompany returns the company the row names before it runs, by
external id and then domain. Nil when there is none yet.
*/
func priorCompany(ctx context.Context, q *models.Queries, ws ids.UUID, r record) (*prior, error) {
	var (
		c   models.Company
		err = database.ErrNotFound
	)

	if r.self.ExternalID != nil {
		c, err = q.FindCompanyByExternalID(ctx, models.FindCompanyByExternalIDParams{
			WorkspaceID: ws, ExternalID: r.self.ExternalID,
		})
	}

	if domain := companies.NormalizeDomain(r.self.Domain); errors.Is(err, database.ErrNotFound) && domain != nil {
		c, err = q.FindCompanyByDomain(ctx, models.FindCompanyByDomainParams{WorkspaceID: ws, Domain: domain})
	}

	if err != nil {
		if found, err := present(err); !found {
			return nil, err
		}
	}

	fields, err := companySnapshot(c)
	if err != nil {
		return nil, err
	}

	return &prior{id: c.ID, fields: fields}, nil
}

/*
change is what one row did to one record, and the company a customer
goes in when the row names it only by name.
*/
type change struct {
	id          ids.UUID
	created     bool
	before      *prior
	after       snapshot
	tags        []ids.UUID
	companyName *string
}

/*
keep saves what the row did, when the import is running for real and
the row changed anything. A record found by other keys than the ones
read before keeps only its new tags, as its old values are unknown.
*/
func (a applier) keep(ctx context.Context, q *models.Queries, c change) error {
	if a.importID == ids.Nil() {
		return nil
	}

	before, after := snapshot{}, c.after
	if !c.created {
		after = snapshot{}
		if c.before != nil && c.before.id == c.id {
			before, after = diff(c.before.fields, c.after)
		}
	}

	if !c.created && len(after) == 0 && len(c.tags) == 0 {
		return nil
	}

	b, err := json.Marshal(before)
	if err != nil {
		return fmt.Errorf("encode change: %w", err)
	}

	af, err := json.Marshal(after)
	if err != nil {
		return fmt.Errorf("encode change: %w", err)
	}

	err = q.RecordImportChange(ctx, models.RecordImportChangeParams{
		WorkspaceID: a.m.Workspace.ID, ImportID: a.importID, RecordID: c.id, Created: c.created,
		Before: b, After: af, AddedTagIds: append([]ids.UUID{}, c.tags...), Now: a.now(),
	})
	if err != nil {
		return fmt.Errorf("record import change: %w", err)
	}

	return nil
}

/*
keepContact reads the customer as the row left them and saves what the
row changed.
*/
func (a applier) keepContact(ctx context.Context, q *models.Queries, c change) error {
	if a.importID == ids.Nil() {
		return nil
	}

	row, err := q.FindContact(ctx, models.FindContactParams{WorkspaceID: a.m.Workspace.ID, ID: c.id})
	if err != nil {
		return fmt.Errorf("find imported contact: %w", err)
	}

	if c.after, err = contactSnapshot(row); err != nil {
		return err
	}

	return a.keep(ctx, q, c)
}
