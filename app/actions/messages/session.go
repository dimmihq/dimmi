/*
|--------------------------------------------------------------------------
| Writing Sessions
|--------------------------------------------------------------------------
|
| Every request that writes to a timeline works the same way: it locks
| the conversation, so writes to one conversation happen one after
| another, writes as the caller, and answers with the entry and the
| conversation as they then are. A request retried with the same client
| message id finds the entry the first one wrote under its dedupe key
| and answers with that instead.
|
*/

package messages

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
excerptLength is how many characters of a message a summary shows.
*/
const excerptLength = 140

/*
session is one request writing to a conversation: the writer acting as
the caller, the locked conversation, the caller's membership and the
entry written.
*/
type session struct {
	w   *conversations.Writer
	c   models.Conversation
	m   authz.Membership
	msg models.Message
}

/*
target is the conversation a request writes to, who writes, and the id
the client gave the change.
*/
type target struct {
	m        authz.Membership
	ref      conversations.Ref
	mutation string
}

/*
open starts a session on the conversation, locking it.
*/
func open(ctx context.Context, tx *database.Tx, s conversations.Services, t target) (*session, error) {
	w, err := conversations.NewWriter(ctx, tx, s, t.m)
	if err != nil {
		return nil, err
	}

	w.MutationID = t.mutation

	c, err := conversations.Lock(ctx, w.Q, t.m.Workspace.ID, t.ref)
	if err != nil {
		return nil, err
	}

	return &session{w: w, c: c, m: t.m}, nil
}

/*
dedupeKey returns the key under which the caller's entry of the kind
is kept in the conversation, nil without a client message id.
*/
func dedupeKey(kind string, c models.Conversation, m authz.Membership, clientID *string) *string {
	if clientID == nil {
		return nil
	}

	key := fmt.Sprintf("%s:%s:%s:%s", kind, c.ID, m.Member.ID, *clientID)

	return &key
}

/*
replay returns the entry already written under the key, marked as
replayed, or nothing when there is none.
*/
func (s *session) replay(ctx context.Context, key *string) (Written, error) {
	if key == nil {
		return Written{}, nil
	}

	msg, err := s.w.Q.FindMessageByDedupeKey(ctx, models.FindMessageByDedupeKeyParams{
		WorkspaceID: s.c.WorkspaceID, DedupeKey: key,
	})
	if errors.Is(err, database.ErrNotFound) {
		return Written{}, nil
	}

	if err != nil {
		return Written{}, fmt.Errorf("find earlier entry: %w", err)
	}

	s.msg = msg

	out, err := s.written(ctx)
	out.Replayed = true

	return out, err
}

/*
written returns the entry written with the conversation as it now is
and its author.
*/
func (s *session) written(ctx context.Context) (Written, error) {
	loaded, err := conversations.LoadOne(ctx, s.w.Q, s.m.Member.ID, s.c)
	if err != nil {
		return Written{}, err
	}

	members, err := conversations.Members(ctx, s.w.Q, s.c.WorkspaceID, []ids.UUID{s.m.Member.ID})
	if err != nil {
		return Written{}, fmt.Errorf("load author: %w", err)
	}

	return Written{Message: s.msg, Conversation: loaded, Author: members[s.m.Member.ID]}, nil
}

/*
summaries describes the entries the way a refusal lists them: id, who
wrote them, the start of what they said and when.
*/
func (s *session) summaries(ctx context.Context, entries []models.Message) []map[string]any {
	var memberIDs []ids.UUID

	for _, e := range entries {
		if e.AuthorType == string(enums.AuthorTypeMember) && e.AuthorID != nil {
			memberIDs = append(memberIDs, *e.AuthorID)
		}
	}

	members, _ := conversations.Members(ctx, s.w.Q, s.c.WorkspaceID, memberIDs)
	contact, _ := s.w.Q.FindContact(ctx, models.FindContactParams{WorkspaceID: s.c.WorkspaceID, ID: s.c.ContactID})

	out := make([]map[string]any, 0, len(entries))
	for _, e := range entries {
		name := ""
		if e.AuthorType == string(enums.AuthorTypeMember) && e.AuthorID != nil {
			name = members[*e.AuthorID].User.Name
		} else if e.AuthorType == string(enums.AuthorTypeContact) && contact.Name != nil {
			name = *contact.Name
		}

		out = append(out, map[string]any{
			"id": models.PublicID(models.PrefixMessage, e.ID), "object": "message", "type": e.Type,
			"author":  map[string]any{"type": e.AuthorType, "name": name},
			"excerpt": excerpt(e.Body), "created_at": e.CreatedAt,
		})
	}

	return out
}

/*
excerpt returns the start of a message, cut short where it is long.
*/
func excerpt(body *string) string {
	if body == nil {
		return ""
	}

	runes := []rune(*body)
	if len(runes) <= excerptLength {
		return *body
	}

	return string(runes[:excerptLength]) + "…"
}

/*
consume ends the caller's draft of the conversation in the mode the
message was written in.
*/
func (s *session) consume(ctx context.Context, mode string) error {
	return conversations.ConsumeDraft(ctx, s.w, conversations.SentDraft{
		Workspace: s.c.WorkspaceID, Conversation: &s.c.ID, Mode: mode,
	})
}
