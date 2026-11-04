/*
|--------------------------------------------------------------------------
| After The Message Is Stored
|--------------------------------------------------------------------------
|
| The email channel's own records are written in the same transaction
| as the message, so they never disagree with it: the Message-IDs that
| thread future replies, including those the email only referenced,
| the conversation's reply token, and the received email's outcome.
|
| A conversation that bulk mail opened is closed straight away, and a
| teammate's forward leaves an internal note with their comment, so the
| team knows who handed the customer over.
|
*/

package email

import (
	"context"
	"fmt"
	"slices"

	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	inbound "github.com/dimmihq/dimmi/app/integrations/email"
	"github.com/dimmihq/dimmi/app/models"
)

/*
maxReferencedRefs bounds the referenced ids filed for one message.
*/
const maxReferencedRefs = 20

/*
persisting is a stored message's email side: the email it came from,
what was learned and decided about it, its Message-ID, and once stored,
the queries of the transaction and the conversation and message.
*/
type persisting struct {
	mail  *received
	insp  inspection
	plan  plan
	ownID string
	q     *models.Queries
	conv  models.Conversation
	msg   models.Message
}

/*
afterPersist returns the hook that writes the channel's own rows once
the message is stored.
*/
func (a *ProcessInboundEmail) afterPersist(p persisting) contracts.AfterPersist {
	return func(ctx context.Context, tx *database.Tx, conv models.Conversation, msg models.Message) error {
		p.q, p.conv, p.msg = models.New(tx), conv, msg

		steps := []func(context.Context, persisting) error{a.fileRefs, a.ensureThread, a.settle, a.markOutcome}
		for _, step := range steps {
			if err := step(ctx, p); err != nil {
				return err
			}
		}

		return nil
	}
}

/*
fileRefs files the email's own Message-ID under the conversation, and
the ids it references that the workspace has not seen yet, so their
mail finds this conversation when it arrives later.
*/
func (a *ProcessInboundEmail) fileRefs(ctx context.Context, p persisting) error {
	err := p.q.CreateEmailMessageRef(ctx, models.CreateEmailMessageRefParams{
		WorkspaceID: p.conv.WorkspaceID, RfcMessageID: p.ownID, ConversationID: p.conv.ID, MessageID: &p.msg.ID,
		Kind: string(enums.EmailRefKindInbound), Now: a.Clock.Now(),
	})
	if err != nil {
		return fmt.Errorf("file message id: %w", err)
	}

	parsed := p.mail.parsed
	referenced := slices.Compact(slices.Concat(parsed.InReplyTo, parsed.References))

	for _, id := range referenced[:min(len(referenced), maxReferencedRefs)] {
		if id == p.ownID {
			continue
		}

		err := p.q.CreateEmailMessageRef(ctx, models.CreateEmailMessageRefParams{
			WorkspaceID: p.conv.WorkspaceID, RfcMessageID: id, ConversationID: p.conv.ID,
			Kind: string(enums.EmailRefKindReferenced), Now: a.Clock.Now(),
		})
		if err != nil {
			return fmt.Errorf("file referenced id: %w", err)
		}
	}

	return nil
}

/*
ensureThread gives the conversation a reply token for the address the
email reached, keeping the one it has.
*/
func (a *ProcessInboundEmail) ensureThread(ctx context.Context, p persisting) error {
	parsed := p.mail.parsed

	_, err := p.q.EnsureEmailThread(ctx, models.EnsureEmailThreadParams{
		WorkspaceID: p.conv.WorkspaceID, ConversationID: p.conv.ID, EmailAddressID: &p.mail.address.ID,
		ReplyToken:        inbound.NewReplyToken(),
		RootMessageID:     optional(inbound.ThreadRoot(parsed.References, parsed.InReplyTo, p.ownID)),
		NormalizedSubject: optional(inbound.NormalizeSubject(parsed.Subject)), Now: a.Clock.Now(),
	})
	if err != nil {
		return fmt.Errorf("ensure email thread: %w", err)
	}

	return nil
}

/*
settle applies the plan's effects on the stored message: why nothing may
answer it, closing a conversation bulk mail opened, and the note of a
teammate's forward.
*/
func (a *ProcessInboundEmail) settle(ctx context.Context, p persisting) error {
	if p.plan.aiSkip != "" {
		err := p.q.AddEmailMessageMetadata(ctx, models.AddEmailMessageMetadataParams{
			Metadata: jsonOf(map[string]string{"ai_skip": p.plan.aiSkip}), Now: a.Clock.Now(),
			WorkspaceID: p.msg.WorkspaceID, ID: p.msg.ID,
		})
		if err != nil {
			return fmt.Errorf("mark message unanswerable: %w", err)
		}
	}

	if p.plan.closeNew && p.conv.MessageCount == 1 {
		err := p.q.CloseEmailConversation(ctx, models.CloseEmailConversationParams{
			Now: a.Clock.Now(), WorkspaceID: p.conv.WorkspaceID, ID: p.conv.ID,
		})
		if err != nil {
			return fmt.Errorf("close bulk conversation: %w", err)
		}
	}

	if p.insp.forward == nil || p.insp.member == nil {
		return nil
	}

	return a.forwardNote(ctx, p)
}

/*
forwardNote leaves an internal note saying which teammate forwarded the
customer's mail, with what they wrote above it.
*/
func (a *ProcessInboundEmail) forwardNote(ctx context.Context, p persisting) error {
	body := "Forwarded by " + p.insp.member.Name
	if p.insp.forward.Comment != "" {
		body += ": " + p.insp.forward.Comment
	}

	if p.insp.forward.Email == "" {
		body += "\n\nThe original sender's address was missing; add their email to reply."
	}

	_, err := p.q.CreateMessage(ctx, models.CreateMessageParams{
		WorkspaceID: p.conv.WorkspaceID, ConversationID: p.conv.ID, Type: string(enums.MessageTypeNote),
		AuthorType: string(enums.AuthorTypeMember), AuthorID: &p.insp.member.ID, Body: &body, Substantive: false,
		Metadata: []byte(`{}`), Now: a.Clock.Now(),
	})
	if err != nil {
		return fmt.Errorf("note forward: %w", err)
	}

	return nil
}

/*
markOutcome notes that mail reached the address and records the
message the received email became.
*/
func (a *ProcessInboundEmail) markOutcome(ctx context.Context, p persisting) error {
	err := p.q.RecordEmailAddressInbound(ctx, models.RecordEmailAddressInboundParams{
		Now: a.Clock.Now(), WorkspaceID: p.mail.address.WorkspaceID, ID: p.mail.address.ID,
	})
	if err != nil {
		return fmt.Errorf("record address inbound: %w", err)
	}

	err = p.q.FinishInboundEmail(ctx, models.FinishInboundEmailParams{
		Status: string(p.plan.status), ConversationID: &p.conv.ID, MessageID: &p.msg.ID, Now: a.Clock.Now(),
		WorkspaceID: p.mail.row.WorkspaceID, ID: p.mail.row.ID,
	})
	if err != nil {
		return fmt.Errorf("finish inbound email: %w", err)
	}

	return nil
}
