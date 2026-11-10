/*
|--------------------------------------------------------------------------
| Teammate Echo
|--------------------------------------------------------------------------
|
| On a chat app the team answers where the customer asked, not only in
| the inbox. A linked teammate's reply typed natively in a conversation's
| thread arrives like any inbound message, but it is the team's answer:
| it is stored as that teammate's reply, already delivered, so it
| answers the conversation, stops the agent, and is never sent anywhere.
| A teammate starting a thread of their own is no conversation, so an
| echo without one is refused and the channel drops it.
|
*/

package conversations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ErrEchoWithoutConversation means a teammate's native reply arrived on a
thread that holds no conversation.
*/
var ErrEchoWithoutConversation = errors.New("inbox: teammate reply outside any conversation")

/*
echo stores the teammate's native reply in the conversation of its
thread, then runs the channel's hook.
*/
func (run *ingestRun) echo(ctx context.Context, conn models.ChannelConnection) (contracts.InboundResult, error) {
	found, err := run.existing(ctx, conn)
	if err != nil {
		return contracts.InboundResult{}, err
	}

	if found == nil {
		return contracts.InboundResult{}, ErrEchoWithoutConversation
	}

	metadata, err := json.Marshal(map[string]any{"echo": true, conn.Type: run.msg.Metadata})
	if err != nil {
		return contracts.InboundResult{}, fmt.Errorf("encode echo metadata: %w", err)
	}

	sentAt := run.msg.SentAt
	member := run.w.AsMember(*run.msg.Echo.MemberID, run.msg.FromMemberName)

	message, conv, err := member.Append(ctx, *found, NewMessage{
		Type: string(enums.MessageTypeMessage), Body: run.msg.Body, BodyFormat: optional(string(run.msg.BodyFormat)),
		ChannelMessageID: optional(run.msg.ChannelMessageID), DedupeKey: &run.msg.DedupeKey,
		DeliveryStatus: new(string(enums.DeliveryStatusSent)), Metadata: metadata, SentAt: &sentAt,
	})
	if err != nil {
		return contracts.InboundResult{}, err
	}

	result := contracts.InboundResult{MessageID: message.ID, ConversationID: conv.ID, ContactID: conv.ContactID}
	if run.msg.AfterPersist == nil {
		return result, nil
	}

	return result, run.msg.AfterPersist(ctx, run.tx, conv, message)
}
