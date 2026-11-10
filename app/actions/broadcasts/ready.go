/*
|--------------------------------------------------------------------------
| Ready To Send
|--------------------------------------------------------------------------
|
| Pressing send checks everything a broadcast needs before anyone gets
| it: a message, a sender who still works here, an audience chosen on
| purpose, and what its channel needs, such as a connected address on a
| verified domain for email. Each problem says how to fix it.
|
*/

package broadcasts

import (
	"context"
	"strings"

	"github.com/dimmihq/dimmi/framework/fault"

	"github.com/dimmihq/dimmi/app/models"
)

/*
What can stop a broadcast from sending.
*/
var (
	ErrNoBody    = fault.New(fault.Invalid, "broadcast_body_missing", "Write the message before sending it.")
	ErrNoSubject = fault.New(fault.Invalid, "broadcast_subject_missing",
		"Give the email a subject before sending it.")
	ErrNoAudience = fault.New(fault.Invalid, "broadcast_audience_missing",
		"Choose who gets this broadcast before sending it.")
	ErrNoSender = fault.New(fault.Invalid, "broadcast_sender_missing",
		"Choose an active teammate the broadcast comes from.")
	ErrUnknownVariable = fault.New(fault.Invalid, "broadcast_variable_unknown",
		"The message uses a detail dimmi doesn't know. Use contact, company, sender or workspace details.")
)

/*
ready returns the first reason the broadcast cannot be sent yet.
*/
func (b *Broadcasts) ready(ctx context.Context, q *models.Queries, row models.Broadcast) error {
	if strings.TrimSpace(row.Body) == "" {
		return ErrNoBody
	}

	if row.Channel == ChannelEmail && strings.TrimSpace(row.Subject) == "" {
		return ErrNoSubject
	}

	if row.AudienceID == nil && len(nullJSON(row.Rules)) == 0 && len(row.RoomIds) == 0 && row.Channel != ChannelSlack {
		return ErrNoAudience
	}

	if err := knownVariables(row.Subject + row.Preview + row.Body); err != nil {
		return err
	}

	if err := senderReady(ctx, q, row); err != nil {
		return err
	}

	sender, ok := b.Senders[row.Channel]
	if !ok {
		return ErrChannel
	}

	return sender.Ready(ctx, q, row)
}

/*
senderReady checks the teammate the broadcast comes from still works in
the workspace.
*/
func senderReady(ctx context.Context, q *models.Queries, row models.Broadcast) error {
	if row.SenderMemberID == nil {
		return ErrNoSender
	}

	sender, err := q.FindBroadcastSender(ctx, models.FindBroadcastSenderParams{
		WorkspaceID: row.WorkspaceID, ID: *row.SenderMemberID,
	})
	if err != nil || sender.Status != "active" {
		return ErrNoSender
	}

	return nil
}

/*
knownVariables refuses a variable outside the details dimmi fills in.
*/
func knownVariables(text string) error {
	for _, m := range variable.FindAllStringSubmatch(text, -1) {
		scope, _, _ := strings.Cut(m[1], ".")
		switch scope {
		case "contact", "company", "sender", "workspace":
		default:
			return ErrUnknownVariable.With("variable", m[1])
		}
	}

	return nil
}
