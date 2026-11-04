/*
|--------------------------------------------------------------------------
| Search
|--------------------------------------------------------------------------
|
| The quick search behind Cmd+K: a few conversations whose subject or
| messages match, newest activity first, each with an excerpt of the
| matching message, and a few contacts by name, address or id. A number
| or a conversation's id jumps straight to that conversation. Search
| covers every status but never spam or deleted conversations, and only
| what the caller may see; contacts show only to those who may read
| them.
|
*/

package conversations

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/contacts"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/querybuilder"
	"github.com/dimmihq/dimmi/app/support/searchquery"
)

/*
SearchInput is who searches for what, and how many of each kind of
result they want.
*/
type SearchInput struct {
	Membership authz.Membership
	Query      string
	Limit      int
}

/*
Hit is a conversation a search found, with an excerpt of the newest
matching message, marks around the matched words, and that message's
id; both are empty when only the subject matched.
*/
type Hit struct {
	Conversation Loaded
	Highlight    string
	MessageID    *ids.UUID
}

/*
SearchResults are what a search found: the conversation it jumps to,
if any, then conversations and contacts.
*/
type SearchResults struct {
	Jump          *Loaded
	Conversations []Hit
	Contacts      []models.Contact
}

/*
Search searches conversations and contacts.

Rules:
  - The caller needs conversations:read; contacts show only with
    contacts:read.
  - A search longer than 256 characters is refused.

Errors: [exceptions.ErrForbidden], [exceptions.ErrSearchQueryTooLong],
or a wrapped database error.
*/
type Search struct {
	DB  *database.DB
	IDs *ids.Registry
}

/*
Handle runs the search.
*/
func (a *Search) Handle(ctx context.Context, in SearchInput) (SearchResults, error) {
	if err := CanRead(in.Membership); err != nil {
		return SearchResults{}, err
	}

	if len([]rune(in.Query)) > searchquery.MaxLength {
		return SearchResults{}, exceptions.ErrSearchQueryTooLong
	}

	sq := searchquery.Parse(in.Query)

	var out SearchResults

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		s := searcher{tx: tx, q: models.New(tx), m: in.Membership, limit: in.Limit}

		var err error
		if out.Jump, err = s.jump(ctx, a.IDs, sq); err != nil || out.Jump != nil {
			return err
		}

		if out.Conversations, err = s.conversations(ctx, sq); err != nil {
			return err
		}

		out.Contacts, err = s.contacts(ctx, in.Query, sq)

		return err
	})

	return out, err
}

/*
searcher runs the parts of one search in its transaction.
*/
type searcher struct {
	tx    *database.Tx
	q     *models.Queries
	m     authz.Membership
	limit int
}

/*
jump returns the conversation a number or id names, nil when the search
names none or one the caller cannot see.
*/
func (s searcher) jump(ctx context.Context, registry *ids.Registry, sq searchquery.Query) (*Loaded, error) {
	var ref Ref

	switch {
	case sq.Number != nil:
		ref.Number = *sq.Number
	case strings.HasPrefix(sq.ID, string(models.PrefixConversation)+"_"):
		ref.ID = decodeConversation(registry, sq.ID)
	default:
		return nil, nil
	}

	if ref == (Ref{}) {
		return nil, nil
	}

	c, err := Find(ctx, s.q, s.m.Workspace.ID, ref)
	if err != nil {
		return nil, ignoreNotFound(err)
	}

	loaded, err := LoadOne(ctx, s.q, s.m.Member.ID, c)

	return &loaded, err
}

/*
decodeConversation reads a conversation's public id, nil when it is
none, which finds nothing.
*/
func decodeConversation(registry *ids.Registry, id string) ids.UUID {
	decoded, err := registry.Decode(string(models.PrefixConversation), id)
	if err != nil {
		return ids.Nil()
	}

	return decoded
}

/*
ignoreNotFound turns a conversation that is not found into no result.
*/
func ignoreNotFound(err error) error {
	if errors.Is(err, exceptions.ErrNotFound) {
		return nil
	}

	return err
}

/*
conversations returns the conversations the words find, with excerpts.
*/
func (s searcher) conversations(ctx context.Context, sq searchquery.Query) ([]Hit, error) {
	if sq.TSQuery == "" {
		return nil, nil
	}

	rows, err := selectConversations(ctx, s.tx, querybuilder.Conversations{
		WorkspaceID: s.m.Workspace.ID, MemberID: s.m.Member.ID, TSQuery: &sq.TSQuery, Statuses: sq.Statuses,
		Sort: querybuilder.SortLatest, Limit: s.limit,
	})
	if err != nil {
		return nil, fmt.Errorf("search conversations: %w", err)
	}

	page, err := toPage(ctx, s.q, s.m.Member.ID, rows)
	if err != nil || len(page.Conversations) == 0 {
		return nil, err
	}

	convIDs := make([]ids.UUID, len(page.Conversations))
	for i, l := range page.Conversations {
		convIDs[i] = l.Conversation.ID
	}

	highlights, err := s.q.SearchHighlights(ctx, models.SearchHighlightsParams{
		WorkspaceID: s.m.Workspace.ID, Ids: convIDs, Query: sq.TSQuery,
	})
	if err != nil {
		return nil, fmt.Errorf("highlight matches: %w", err)
	}

	byConversation := byID(highlights, func(h models.SearchHighlightsRow) ids.UUID { return h.ConversationID })

	hits := make([]Hit, len(page.Conversations))
	for i, l := range page.Conversations {
		hits[i] = Hit{Conversation: l}
		if h, ok := byConversation[l.Conversation.ID]; ok {
			hits[i].Highlight, hits[i].MessageID = h.Highlight, &h.ID
		}
	}

	return hits, nil
}

/*
contacts returns the contacts the search finds, when the caller may
read contacts.
*/
func (s searcher) contacts(ctx context.Context, raw string, sq searchquery.Query) ([]models.Contact, error) {
	if !s.m.Can("contacts:read") || strings.TrimSpace(raw) == "" {
		return nil, nil
	}

	rows, err := s.q.ListContacts(ctx, models.ListContactsParams{
		WorkspaceID: s.m.Workspace.ID, Search: contacts.LikePattern(raw), Terms: sq.TSQuery,
		Sort: "last_seen_at", RowLimit: int64(s.limit),
	})
	if err != nil {
		return nil, fmt.Errorf("search contacts: %w", err)
	}

	return rows, nil
}
