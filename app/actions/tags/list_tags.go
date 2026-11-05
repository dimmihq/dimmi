/*
|--------------------------------------------------------------------------
| List Tags
|--------------------------------------------------------------------------
|
| Pickers read the vocabulary by name. The settings page also asks how
| many live records carry each tag, a heavier read kept behind a flag so
| typing in a picker never pays for it.
|
*/

package tags

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Listed is a tag and how many live records carry it, zero unless counts
were asked for.
*/
type Listed struct {
	Tag           models.Tag
	Conversations int64
	Contacts      int64
	Companies     int64
}

/*
ListTagsInput is who asks, a prefix names must start with, and whether
to count the records carrying each tag.
*/
type ListTagsInput struct {
	Membership authz.Membership
	Prefix     *string
	Counts     bool
}

/*
ListTags lists the workspace's tags by name.

Rules:
  - The caller needs conversations:read.

Errors: [exceptions.ErrForbidden], or a wrapped database error.
*/
type ListTags struct {
	DB *database.DB
}

/*
Handle returns the tags.
*/
func (a *ListTags) Handle(ctx context.Context, in ListTagsInput) ([]Listed, error) {
	if !in.Membership.Can("conversations:read") {
		return nil, exceptions.ErrForbidden
	}

	var out []Listed

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q, ws := models.New(tx), in.Membership.Workspace.ID

		rows, err := q.ListTags(ctx, models.ListTagsParams{WorkspaceID: ws, Prefix: in.Prefix})
		if err != nil {
			return fmt.Errorf("list tags: %w", err)
		}

		out = make([]Listed, len(rows))
		for i, row := range rows {
			out[i].Tag = row
		}

		if !in.Counts {
			return nil
		}

		return addCounts(ctx, q, ws, out)
	})

	return out, err
}

/*
addCounts fills in how many live records carry each tag.
*/
func addCounts(ctx context.Context, q *models.Queries, ws ids.UUID, listed []Listed) error {
	byTag := make(map[ids.UUID]*Listed, len(listed))
	for i := range listed {
		byTag[listed[i].Tag.ID] = &listed[i]
	}

	conversations, err := q.CountTaggedConversations(ctx, ws)
	if err != nil {
		return fmt.Errorf("count tagged conversations: %w", err)
	}

	for _, row := range conversations {
		if l, ok := byTag[row.TagID]; ok {
			l.Conversations = row.Conversations
		}
	}

	contacts, err := q.CountTaggedContacts(ctx, ws)
	if err != nil {
		return fmt.Errorf("count tagged contacts: %w", err)
	}

	for _, row := range contacts {
		if l, ok := byTag[row.TagID]; ok {
			l.Contacts = row.Tagged
		}
	}

	companies, err := q.CountTaggedCompanies(ctx, ws)
	if err != nil {
		return fmt.Errorf("count tagged companies: %w", err)
	}

	for _, row := range companies {
		if l, ok := byTag[row.TagID]; ok {
			l.Companies = row.Tagged
		}
	}

	return nil
}
