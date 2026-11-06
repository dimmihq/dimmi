/*
|--------------------------------------------------------------------------
| Resolving Threads
|--------------------------------------------------------------------------
|
| Which conversation a received email continues, strongest evidence
| first: a reply token in an address it was sent to, the message it
| answers, the thread it references from the newest message back, mail
| that mentioned it before it arrived, and as a last resort its subject.
|
| Every lookup is scoped to the workspace the mail was routed to. A
| token or Message-ID of another workspace simply finds nothing, so
| mail can never be pulled into another tenant's conversation. The
| subject is trusted only for a reply-looking subject specific enough
| to mean something, from the same customer, on the same address, to
| exactly one open conversation active in the last two weeks.
|
*/

package email

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/enums"
	inbound "github.com/dimmihq/dimmi/app/integrations/email"
	"github.com/dimmihq/dimmi/app/models"
)

/*
subjectWindow is how recently a conversation must have been active for
the subject heuristic to join it.
*/
const subjectWindow = 14 * 24 * time.Hour

/*
How a thread was found.
*/
const (
	methodToken      = "token"
	methodInReplyTo  = "in_reply_to"
	methodReferences = "references"
	methodReverse    = "reverse_reference"
	methodSubject    = "subject"
	methodNew        = "new"
)

/*
threadHit is the conversation a received email continues, and how it
was found; Conversation is nil for a new thread. Reused marks a new
thread whose Message-ID another conversation's mail already carries,
as careless scripts send every mail under one id.
*/
type threadHit struct {
	conversation *models.Conversation
	method       string
	reused       bool
}

/*
resolveThread finds the conversation the email continues.
*/
func (a *ProcessInboundEmail) resolveThread(
	ctx context.Context, q *models.Queries, mail *received, contact *models.Contact,
) (threadHit, error) {
	steps := []struct {
		method string
		find   func() (*ids.UUID, error)
	}{
		{methodToken, func() (*ids.UUID, error) { return a.byToken(ctx, q, mail) }},
		{methodInReplyTo, func() (*ids.UUID, error) { return byRefs(ctx, q, refsOf(mail, mail.parsed.InReplyTo)) }},
		{methodReferences, func() (*ids.UUID, error) { return byReferences(ctx, q, mail) }},
		{methodReverse, func() (*ids.UUID, error) { return byReverse(ctx, q, mail) }},
		{methodSubject, func() (*ids.UUID, error) { return a.bySubject(ctx, q, mail, contact) }},
	}

	for _, step := range steps {
		id, err := step.find()
		if err != nil {
			return threadHit{}, err
		}

		if id == nil {
			continue
		}

		conversation, err := q.FindConversation(ctx, models.FindConversationParams{
			WorkspaceID: mail.row.WorkspaceID, ID: *id,
		})
		if err != nil {
			return threadHit{}, fmt.Errorf("find conversation: %w", err)
		}

		if conversation.DeletedAt == nil {
			return threadHit{conversation: &conversation, method: step.method}, nil
		}
	}

	if mail.parsed.MessageID == "" {
		return threadHit{method: methodNew}, nil
	}

	reused, err := byRefs(ctx, q, refsOf(mail, []string{mail.parsed.MessageID}))
	if err != nil {
		return threadHit{}, err
	}

	return threadHit{method: methodNew, reused: reused != nil}, nil
}

/*
byToken finds the live thread of a reply token in any address the
email was sent to.
*/
func (a *ProcessInboundEmail) byToken(ctx context.Context, q *models.Queries, mail *received) (*ids.UUID, error) {
	for _, address := range recipientsOf(mail) {
		parsed, ok := inbound.ParseRecipient(address, a.Config.InboundDomain)
		if !ok || parsed.Token == "" {
			continue
		}

		thread, err := q.FindEmailThreadByToken(ctx, models.FindEmailThreadByTokenParams{
			WorkspaceID: mail.row.WorkspaceID, ReplyToken: parsed.Token,
		})
		if errors.Is(err, database.ErrNotFound) {
			continue
		}

		if err != nil {
			return nil, fmt.Errorf("find thread by token: %w", err)
		}

		return &thread.ConversationID, nil
	}

	return nil, nil
}

/*
byReferences looks the referenced ids up one by one, newest first.
*/
func byReferences(ctx context.Context, q *models.Queries, mail *received) (*ids.UUID, error) {
	for _, id := range inbound.ReferenceLookups(mail.parsed.InReplyTo, mail.parsed.References) {
		found, err := byRefs(ctx, q, refsOf(mail, []string{id}))
		if err != nil || found != nil {
			return found, err
		}
	}

	return nil, nil
}

/*
byReverse finds the conversation of a reply that arrived before this
email and referenced it.
*/
func byReverse(ctx context.Context, q *models.Queries, mail *received) (*ids.UUID, error) {
	if mail.parsed.MessageID == "" {
		return nil, nil
	}

	params := refsOf(mail, []string{mail.parsed.MessageID})
	params.Kinds = []string{string(enums.EmailRefKindReferenced)}

	return byRefs(ctx, q, params)
}

/*
refsOf returns the lookup of the ids among every kind of filed id of
the email's workspace.
*/
func refsOf(mail *received, refs []string) models.FindRefConversationsParams {
	return models.FindRefConversationsParams{
		WorkspaceID: mail.row.WorkspaceID, RfcMessageIds: refs, Kinds: allRefKinds(),
	}
}

/*
byRefs finds the conversation any of the ids is filed under: an open
one first, then the one most recently active.
*/
func byRefs(ctx context.Context, q *models.Queries, params models.FindRefConversationsParams) (*ids.UUID, error) {
	if len(params.RfcMessageIds) == 0 {
		return nil, nil
	}

	rows, err := q.FindRefConversations(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("find conversations by reference: %w", err)
	}

	if len(rows) == 0 {
		return nil, nil
	}

	best := slices.MaxFunc(rows, func(x, y models.FindRefConversationsRow) int {
		if c := cmp.Compare(openness(x.Status), openness(y.Status)); c != 0 {
			return c
		}

		return latest(x.LastMessageAt).Compare(latest(y.LastMessageAt))
	})

	return &best.ID, nil
}

/*
bySubject joins the one open conversation of the same customer on the
same address whose subject matches, when the subject can be trusted.
*/
func (a *ProcessInboundEmail) bySubject(
	ctx context.Context, q *models.Queries, mail *received, contact *models.Contact,
) (*ids.UUID, error) {
	replying := inbound.HasReplyPrefix(mail.parsed.Subject) || len(mail.parsed.InReplyTo) > 0 ||
		len(mail.parsed.References) > 0
	normalized := inbound.NormalizeSubject(mail.parsed.Subject)

	if contact == nil || !replying || !inbound.SpecificSubject(normalized) {
		return nil, nil
	}

	candidates, err := q.FindSubjectThreadCandidates(ctx, models.FindSubjectThreadCandidatesParams{
		WorkspaceID: mail.row.WorkspaceID, ChannelConnectionID: mail.address.ChannelConnectionID,
		ContactID: contact.ID, Since: a.Clock.Now().Add(-subjectWindow), NormalizedSubject: normalized,
	})
	if err != nil {
		return nil, fmt.Errorf("find conversations by subject: %w", err)
	}

	if len(candidates) != 1 {
		return nil, nil
	}

	return &candidates[0], nil
}

/*
allRefKinds lists every kind of filed Message-ID.
*/
func allRefKinds() []string {
	return []string{
		string(enums.EmailRefKindInbound), string(enums.EmailRefKindOutbound),
		string(enums.EmailRefKindOutboundProvider), string(enums.EmailRefKindReferenced),
	}
}

/*
openness ranks a conversation that is not closed above a closed one.
*/
func openness(status string) int {
	if status == string(enums.ConversationStatusClosed) {
		return 0
	}

	return 1
}

/*
latest returns the time, or the zero time when there is none.
*/
func latest(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}

	return *t
}

/*
recipientsOf lists every address the email was sent to: its envelope
recipients, then the addresses its header names.
*/
func recipientsOf(mail *received) []string {
	out := slices.Clone(mail.row.EnvelopeTo)
	for _, field := range []string{"Delivered-To", "X-Original-To", "X-Forwarded-To", "X-Forwarded-For"} {
		if value := mail.parsed.Header.Get(field); value != "" {
			out = append(out, lower(value))
		}
	}

	return append(out, addressesOf(slices.Concat(mail.parsed.To, mail.parsed.Cc))...)
}
