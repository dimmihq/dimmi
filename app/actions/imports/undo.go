/*
|--------------------------------------------------------------------------
| Undoing Records
|--------------------------------------------------------------------------
|
| The changes an import kept are read back record by record. A record
| the import made is removed only while it stands exactly as the import
| left it: same fields, same tags, no conversation and, for a company,
| nobody in it. A record the import changed gets back each field that
| still holds the imported value; a field changed again since keeps its
| new value. The tags the import added come off either way. Removing and
| changing go through the same actions a teammate uses, as the teammate
| undoing the import, so their permissions and events apply.
|
*/

package imports

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/companies"
	"github.com/dimmihq/dimmi/app/actions/contacts"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
The outcomes of undoing a record.
*/
const (
	undoRemoved  = "removed"
	undoReverted = "reverted"
	undoKept     = "kept"
)

/*
The reasons a record is kept.
*/
const (
	keptChanged       = "Changed since the import."
	keptTagged        = "Tagged since the import."
	keptGone          = "Deleted or merged since the import."
	keptConversations = "Has had a conversation since the import."
	keptPeople        = "Has people in it now."
)

/*
undoRecord is everything an import did to one record: whether it made
it, each field's value before the first row and after the last, and
the tags it added.
*/
type undoRecord struct {
	id      ids.UUID
	created bool
	before  snapshot
	after   snapshot
	tags    []ids.UUID
}

/*
group folds an import's changes into one per record, in the order the
records were first touched.
*/
func group(changes []models.ImportChange) ([]undoRecord, error) {
	var out []undoRecord

	at := map[ids.UUID]int{}

	for _, c := range changes {
		var before, after snapshot
		if err := json.Unmarshal(c.Before, &before); err != nil {
			return nil, fmt.Errorf("read import change: %w", err)
		}

		if err := json.Unmarshal(c.After, &after); err != nil {
			return nil, fmt.Errorf("read import change: %w", err)
		}

		i, seen := at[c.RecordID]
		if !seen {
			at[c.RecordID], i = len(out), len(out)
			out = append(out, undoRecord{id: c.RecordID, before: snapshot{}, after: snapshot{}})
		}

		out[i].fold(c, before, after)
	}

	return out, nil
}

/*
fold adds one change to the record: a field keeps its first value
before and takes its last value after.
*/
func (r *undoRecord) fold(c models.ImportChange, before, after snapshot) {
	r.created = r.created || c.Created

	for k, v := range before {
		if _, had := r.before[k]; !had {
			r.before[k] = v
		}
	}

	maps.Copy(r.after, after)

	for _, t := range c.AddedTagIds {
		if !slices.Contains(r.tags, t) {
			r.tags = append(r.tags, t)
		}
	}
}

/*
undone is what undoing one record came to, with the record's name and
why it was kept.
*/
type undone struct {
	outcome string
	name    string
	reason  string
}

/*
undoer undoes records as one teammate.
*/
type undoer struct {
	imports *Imports
	m       authz.Membership
	kind    string
}

/*
publicID returns the record's public id.
*/
func (u undoer) publicID(id ids.UUID) string {
	if u.kind == KindCompanies {
		return models.PublicID(models.PrefixCompany, id)
	}

	return models.PublicID(models.PrefixContact, id)
}

/*
undo undoes one record. A refusal a person can read keeps the record,
with that refusal as the reason; anything else stops the undo.
*/
func (u undoer) undo(ctx context.Context, r undoRecord) (undone, error) {
	var (
		o   undone
		err error
	)

	if u.kind == KindCompanies {
		o, err = u.company(ctx, r)
	} else {
		o, err = u.contact(ctx, r)
	}

	if err == nil {
		return o, nil
	}

	if message := reason(err); message != "could not be saved" {
		return undone{outcome: undoKept, name: o.name, reason: message}, nil
	}

	return o, err
}

/*
contact undoes what the import did to one customer.
*/
func (u undoer) contact(ctx context.Context, r undoRecord) (undone, error) {
	st, err := u.contactNow(ctx, models.New(u.imports.DB), r.id)
	if err != nil || st.gone {
		return kept(st.o, keptGone), err
	}

	if r.created {
		return u.removeContact(ctx, r, st)
	}

	back, conflict := revert(r, st.fields)
	remove := stillOn(r.tags, st.tags)

	if len(back) == 0 && len(remove) == 0 {
		return settled(st.o, conflict), nil
	}

	in := contactRevert(back)
	in.Membership, in.ContactID, in.RemoveTags = u.m, r.id, remove

	_, err = u.imports.UpdateContact.Handle(ctx, in)
	st.o.outcome = undoReverted

	return st.o, err
}

/*
removeContact removes a customer the import made, unless someone has
worked on them since.
*/
func (u undoer) removeContact(ctx context.Context, r undoRecord, st standing) (undone, error) {
	if why := touched(r, st); why != "" {
		return kept(st.o, why), nil
	}

	err := u.imports.DeleteContact.Handle(ctx, contacts.ShowContactInput{Membership: u.m, ContactID: r.id})
	if errors.Is(err, exceptions.ErrContactHasConversations) {
		return kept(st.o, keptConversations), nil
	}

	st.o.outcome = undoRemoved

	return st.o, err
}

/*
company undoes what the import did to one company.
*/
func (u undoer) company(ctx context.Context, r undoRecord) (undone, error) {
	st, err := u.companyNow(ctx, models.New(u.imports.DB), r.id)
	if err != nil || st.gone {
		return kept(st.o, keptGone), err
	}

	if r.created {
		return u.removeCompany(ctx, r, st)
	}

	back, conflict := revert(r, st.fields)
	remove := stillOn(r.tags, st.tags)

	if len(back) == 0 && len(remove) == 0 {
		return settled(st.o, conflict), nil
	}

	in := companyRevert(back)
	in.Membership, in.CompanyID, in.RemoveTags = u.m, r.id, remove

	_, err = u.imports.UpdateCompany.Handle(ctx, in)
	st.o.outcome = undoReverted

	return st.o, err
}

/*
removeCompany removes a company the import made, unless someone has
worked on it or put people in it since.
*/
func (u undoer) removeCompany(ctx context.Context, r undoRecord, st standing) (undone, error) {
	if why := touched(r, st); why != "" {
		return kept(st.o, why), nil
	}

	people, err := models.New(u.imports.DB).CountCompanyPeople(ctx, models.CountCompanyPeopleParams{
		WorkspaceID: u.m.Workspace.ID, CompanyIds: []ids.UUID{r.id},
	})
	if err != nil {
		return st.o, fmt.Errorf("count company people: %w", err)
	}

	if len(people) > 0 && people[0].People > 0 {
		return kept(st.o, keptPeople), nil
	}

	st.o.outcome = undoRemoved

	return st.o, u.imports.DeleteCompany.Handle(ctx, companies.ShowInput{Membership: u.m, CompanyID: r.id})
}

/*
revert returns the fields to put back: each one that still holds what
the import wrote, with its value from before. It also says why, when
some field was changed again since.
*/
func revert(r undoRecord, current snapshot) (snapshot, string) {
	back, conflict := snapshot{}, ""

	for k, after := range r.after {
		switch {
		case same(current[k], r.before[k]):
		case same(current[k], after):
			back[k] = r.before[k]
		default:
			conflict = keptChanged
		}
	}

	return back, conflict
}

/*
stillOn returns the tags that are still on the record.
*/
func stillOn(added, on []ids.UUID) []ids.UUID {
	var out []ids.UUID

	for _, t := range added {
		if slices.Contains(on, t) {
			out = append(out, t)
		}
	}

	return out
}

/*
sameSet reports whether two lists of tags hold the same ones.
*/
func sameSet(a, b []ids.UUID) bool {
	if len(a) != len(b) {
		return false
	}

	for _, t := range a {
		if !slices.Contains(b, t) {
			return false
		}
	}

	return true
}

/*
kept keeps the record with the reason.
*/
func kept(o undone, why string) undone {
	o.outcome, o.reason = undoKept, why

	return o
}

/*
settled keeps a record with nothing left to undo when it was changed
again since, and counts it as put back when it already is.
*/
func settled(o undone, conflict string) undone {
	if conflict != "" {
		return kept(o, conflict)
	}

	o.outcome = undoReverted

	return o
}

/*
contactName names a customer for the kept list.
*/
func contactName(c models.Contact) string {
	switch {
	case c.Name != nil && *c.Name != "":
		return *c.Name
	case c.Email != nil:
		return *c.Email
	case c.ExternalID != nil:
		return *c.ExternalID
	}

	return "Unnamed person"
}
