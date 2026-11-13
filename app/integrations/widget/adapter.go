/*
|--------------------------------------------------------------------------
| Widget Channel
|--------------------------------------------------------------------------
|
| A teammate's reply on a messenger conversation needs no provider: it is
| shown live in the visitor's open messenger the moment it is stored, and an
| email follows only when nobody read it. So sending does nothing here, and
| the reply counts as sent once stored. The team never starts a thread on
| the widget: a visitor always speaks first.
|
*/

package widget

import (
	"context"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
maxAttachments is how many files one reply carries to a visitor.
*/
const maxAttachments = 10

/*
Channel is the messenger's channel driver.
*/
type Channel struct{}

/*
Type names the widget connection type.
*/
func (Channel) Type() string {
	return string(enums.ChannelTypeWidget)
}

/*
Capabilities says replies travel live, with files, typing and read
receipts, never start a thread, and draw buttons and cards beside a
message.
*/
func (Channel) Capabilities(models.ChannelConnection) contracts.ChannelCapabilities {
	return contracts.ChannelCapabilities{
		Transport: contracts.TransportRealtime, StartConversation: contracts.StartNever,
		Formatting: contracts.FormatMarkdown, Attachments: true, MaxAttachments: maxAttachments,
		Typing: true, DeliveryReceipts: true, ReadReceipts: true, Rich: true,
	}
}

/*
Send delivers nothing itself, since the reply reaches the visitor through
their stream once stored; the message's own key names it on the channel.
*/
func (Channel) Send(
	_ context.Context, _ models.ChannelConnection, msg contracts.OutboundMessage,
) (contracts.SendResult, error) {
	return contracts.SendResult{ChannelMessageID: msg.IdempotencyKey}, nil
}
