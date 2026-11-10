/*
|--------------------------------------------------------------------------
| Replies To Broadcasts
|--------------------------------------------------------------------------
|
| A broadcast email is sent from a connected address with a reply token
| in its Reply-To, and without a conversation behind it. When an answer
| comes back, by its token or by the broadcast's Message-ID, broadcasts
| open the conversation it belongs to, starting with the broadcast, and
| the answer joins it like any reply. Mail that is no answer to a
| broadcast threads as it always did.
|
*/

package email

import (
	"context"
	"slices"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	inbound "github.com/dimmihq/dimmi/app/integrations/email"
	"github.com/dimmihq/dimmi/app/models"
)

/*
methodBroadcast is how a reply to a broadcast was threaded.
*/
const methodBroadcast = "broadcast"

/*
BroadcastReplies opens the conversation of a reply to a broadcast.
*/
type BroadcastReplies interface {
	/*
		EmailReply returns the conversation of the broadcast the tokens or
		Message-IDs name, opening it with the broadcast as its first
		message, or nil when the mail answers no broadcast.
	*/
	EmailReply(ctx context.Context, tx *database.Tx, ws ids.UUID, refs ReplyRefs) (*models.Conversation, error)
}

/*
ReplyRefs is what a reply names of the mail it answers: the reply
tokens of the addresses it was sent to, and the Message-IDs it quotes.
*/
type ReplyRefs struct {
	Tokens     []string
	MessageIDs []string
}

/*
broadcastReply threads a reply to a broadcast that found no conversation
of its own.
*/
func (a *ProcessInboundEmail) broadcastReply(ctx context.Context, d *decision) error {
	if a.Replies == nil || d.hit.conversation != nil || d.tx == nil {
		return nil
	}

	var tokens []string

	for _, address := range recipientsOf(d.mail) {
		if parsed, ok := inbound.ParseRecipient(address, a.Config.InboundDomain); ok && parsed.Token != "" {
			tokens = append(tokens, parsed.Token)
		}
	}

	parsed := d.mail.parsed
	refs := slices.Concat(parsed.InReplyTo, inbound.ReferenceLookups(parsed.InReplyTo, parsed.References))
	if len(tokens) == 0 && len(refs) == 0 {
		return nil
	}

	conversation, err := a.Replies.EmailReply(ctx, d.tx, d.mail.row.WorkspaceID, ReplyRefs{
		Tokens: tokens, MessageIDs: refs,
	})
	if err != nil || conversation == nil {
		return err
	}

	d.hit = threadHit{conversation: conversation, method: methodBroadcast}

	return nil
}
