/*
|--------------------------------------------------------------------------
| Conversation Context
|--------------------------------------------------------------------------
|
| Everything the panel beside a conversation shows about the customer, in
| one round trip: who they are, their company, the ways they reach the
| team, where this conversation started, the last few conversations they
| had and whether they look like someone already known. Past
| conversations go through the inbox's own list, so nothing the reader
| may not see in the inbox shows here either.
|
*/

package conversations

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/contacts"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/querybuilder"
)

/*
How many past conversations the context shows, and the most it counts.
*/
const (
	pastShown      = 3
	pastCountLimit = 100
)

/*
ContextInput is who asks for the context of which conversation.
*/
type ContextInput struct {
	Membership authz.Membership
	Ref        Ref
}

/*
Context is what is known about a conversation's customer. Source is the
page, browser and device the conversation started from, as the channel
recorded it. Past holds the customer's newest other conversations, and
PastTotal how many they have, counted up to 100.
*/
type Context struct {
	Contact    contacts.Detail
	Identities []models.ContactIdentity
	Source     json.RawMessage
	Past       []Loaded
	PastTotal  int
	Duplicates int
}

/*
ShowContext returns the context of a conversation.

Rules:
  - The caller needs conversations:read and contacts:read.
  - Past conversations exclude the current one, the deleted, spam and
    the trash, and those the caller may not see.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], or a
wrapped database error.
*/
type ShowContext struct {
	DB *database.DB
}

/*
Handle returns the context.
*/
func (a *ShowContext) Handle(ctx context.Context, in ContextInput) (Context, error) {
	if err := CanRead(in.Membership); err != nil {
		return Context{}, err
	}

	if !in.Membership.Can("contacts:read") {
		return Context{}, exceptions.ErrForbidden
	}

	var out Context

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		c, err := Find(ctx, q, in.Membership, in.Ref)
		if err != nil {
			return err
		}

		out, err = loadContext(ctx, tx, in.Membership, c)

		return err
	})

	return out, err
}

/*
loadContext reads each part of the context with one query.
*/
func loadContext(ctx context.Context, tx *database.Tx, m authz.Membership, c models.Conversation) (Context, error) {
	q := models.New(tx)

	contact, err := contacts.Survivor(ctx, q, c.WorkspaceID, c.ContactID)
	if err != nil {
		return Context{}, err
	}

	detail, err := contacts.LoadDetail(ctx, q, contact)
	if err != nil {
		return Context{}, err
	}

	identities, err := q.ListContactIdentities(ctx, models.ListContactIdentitiesParams{
		WorkspaceID: c.WorkspaceID, ContactID: contact.ID,
	})
	if err != nil {
		return Context{}, fmt.Errorf("read identities: %w", err)
	}

	duplicates, err := contacts.FindDuplicates(ctx, q, contact)
	if err != nil {
		return Context{}, err
	}

	list, _ := listOf(m, Filters{View: ViewAll, AnyStatus: true, ContactID: &contact.ID}, querybuilder.SortLatest)

	out := Context{Contact: detail, Identities: identities, Source: c.Source, Duplicates: len(duplicates)}
	out.Past, out.PastTotal, err = pastConversations(ctx, tx, list, c.ID)

	return out, err
}

/*
pastConversations reads the customer's newest conversations through the
inbox's list, leaves the current one out, and counts the rest up to the
limit.
*/
func pastConversations(
	ctx context.Context, tx *database.Tx, list querybuilder.Conversations, current ids.UUID,
) ([]Loaded, int, error) {
	list.Limit = pastCountLimit + 1

	rows, err := selectConversations(ctx, tx, list)
	if err != nil {
		return nil, 0, fmt.Errorf("list past conversations: %w", err)
	}

	others := make([]models.Conversation, 0, len(rows))
	for _, row := range rows {
		if row.ID != current {
			others = append(others, row)
		}
	}

	total := min(len(others), pastCountLimit)

	page, err := toPage(ctx, models.New(tx), list.MemberID, others[:min(len(others), pastShown)])
	if err != nil {
		return nil, 0, err
	}

	return page.Conversations, total, nil
}
