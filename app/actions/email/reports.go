/*
|--------------------------------------------------------------------------
| Notices About Addresses
|--------------------------------------------------------------------------
|
| Some mail to a forwarding address is about the address, not for a
| conversation. A provider's forwarding confirmation is kept for the
| admins to finish, and a forwarding test proves the forward works.
| Neither ever becomes a message.
|
*/

package email

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	inbound "github.com/dimmihq/dimmi/app/integrations/email"
	"github.com/dimmihq/dimmi/app/models"
)

/*
maxDiagnosticLength bounds the provider diagnostic kept and shown.
*/
const maxDiagnosticLength = 300

/*
Ignore reasons of mail that never becomes a message.
*/
const (
	ignoreConfirmation     = "forwarding_confirmation"
	ignoreForwardingTest   = "forwarding_test"
	ignoreBounce           = "bounce"
	ignoreBounceUnmatched  = "bounce_unmatched"
	ignoreComplaint        = "complaint"
	ignoreComplaintNotSpam = "complaint_not_spam"
	unmatchedSuffix        = "_unmatched"
)

/*
recipientStatus is what became of one recipient of a sent email.
*/
type recipientStatus struct {
	Status     string    `json:"status"`
	Code       string    `json:"code,omitempty"`
	Diagnostic string    `json:"diagnostic,omitempty"`
	At         time.Time `json:"at"`
}

/*
storedConfirmation is a forwarding confirmation as kept on its address,
with when it arrived.
*/
type storedConfirmation struct {
	inbound.Confirmation

	ReceivedAt time.Time `json:"received_at"`
}

/*
notice keeps a forwarding confirmation, or notes that a forwarding test
arrived, on the address.
*/
func (a *ProcessInboundEmail) notice(ctx context.Context, mail *received, insp inspection) error {
	if insp.testToken != "" {
		return a.testArrived(ctx, mail, insp.testToken)
	}

	reason := ignoreForwardingTest

	err := within(ctx, a.DB, func(ctx context.Context, q *models.Queries) error {
		if insp.confirmation == nil {
			return q.RecordEmailAddressInbound(ctx, models.RecordEmailAddressInboundParams{
				Now: a.Clock.Now(), WorkspaceID: mail.address.WorkspaceID, ID: mail.address.ID,
			})
		}

		reason = ignoreConfirmation

		stored := jsonOf(storedConfirmation{Confirmation: *insp.confirmation, ReceivedAt: a.Clock.Now()})

		return q.StoreForwardingConfirmation(ctx, models.StoreForwardingConfirmationParams{
			ForwardingConfirmation: stored, Now: a.Clock.Now(), WorkspaceID: mail.address.WorkspaceID,
			ID: mail.address.ID,
		})
	})
	if err != nil {
		return fmt.Errorf("record address notice: %w", err)
	}

	return a.ignore(ctx, mail.row, reason)
}

/*
findSent returns the email the workspace sent with the Message-ID, and
whether there is one.
*/
func findSent(
	ctx context.Context, q *models.Queries, workspaceID ids.UUID, messageID string,
) (models.OutboundEmail, bool, error) {
	if messageID == "" {
		return models.OutboundEmail{}, false, nil
	}

	sent, err := q.FindOutboundEmailByMessageIDHeader(ctx, models.FindOutboundEmailByMessageIDHeaderParams{
		WorkspaceID: workspaceID, MessageIDHeader: messageID,
	})
	if errors.Is(err, database.ErrNotFound) {
		return sent, false, nil
	}

	if err != nil {
		return sent, false, fmt.Errorf("find sent email: %w", err)
	}

	return sent, true, nil
}

/*
diagnostic trims a provider diagnostic to what is shown.
*/
func diagnostic(text string) string {
	text = strings.Join(strings.Fields(text), " ")
	if len(text) > maxDiagnosticLength {
		text = text[:maxDiagnosticLength]
	}

	return text
}
