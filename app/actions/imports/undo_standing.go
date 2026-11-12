/*
|--------------------------------------------------------------------------
| How Records Stand
|--------------------------------------------------------------------------
|
| Before undoing a record, the undo reads how it stands now: its name
| for the kept list, its fields read the same way the import read them,
| and its tags. A record the import made is removed only while it still
| stands exactly as the import left it.
|
*/

package imports

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
)

/*
standing is how a record stands now: its name, its fields and its
tags, or that it is gone.
*/
type standing struct {
	o      undone
	gone   bool
	fields snapshot
	tags   []ids.UUID
}

/*
contactNow reads how the customer stands now.
*/
func (u undoer) contactNow(ctx context.Context, q *models.Queries, id ids.UUID) (standing, error) {
	ws := u.m.Workspace.ID

	c, err := q.FindContact(ctx, models.FindContactParams{WorkspaceID: ws, ID: id})
	if errors.Is(err, database.ErrNotFound) {
		return standing{gone: true}, nil
	}

	if err != nil {
		return standing{}, fmt.Errorf("find contact: %w", err)
	}

	st := standing{o: undone{name: contactName(c)}, gone: c.DeletedAt != nil || c.MergedIntoID != nil}
	if st.fields, err = contactSnapshot(c); err != nil {
		return st, err
	}

	tags, err := q.ListContactTags(ctx, models.ListContactTagsParams{WorkspaceID: ws, ContactIds: []ids.UUID{id}})
	if err != nil {
		return st, fmt.Errorf("list contact tags: %w", err)
	}

	for _, t := range tags {
		st.tags = append(st.tags, t.Tag.ID)
	}

	return st, nil
}

/*
companyNow reads how the company stands now.
*/
func (u undoer) companyNow(ctx context.Context, q *models.Queries, id ids.UUID) (standing, error) {
	ws := u.m.Workspace.ID

	c, err := q.FindCompany(ctx, models.FindCompanyParams{WorkspaceID: ws, ID: id})
	if errors.Is(err, database.ErrNotFound) {
		return standing{gone: true}, nil
	}

	if err != nil {
		return standing{}, fmt.Errorf("find company: %w", err)
	}

	st := standing{o: undone{name: c.Name}, gone: c.DeletedAt != nil}
	if st.fields, err = companySnapshot(c); err != nil {
		return st, err
	}

	tags, err := q.ListCompanyTags(ctx, models.ListCompanyTagsParams{WorkspaceID: ws, CompanyIds: []ids.UUID{id}})
	if err != nil {
		return st, fmt.Errorf("list company tags: %w", err)
	}

	for _, t := range tags {
		st.tags = append(st.tags, t.Tag.ID)
	}

	return st, nil
}

/*
touched says why a record the import made no longer stands as the
import left it, or nothing when it does.
*/
func touched(r undoRecord, st standing) string {
	if !same(st.fields, r.after) {
		return keptChanged
	}

	if !sameSet(st.tags, r.tags) {
		return keptTagged
	}

	return ""
}
