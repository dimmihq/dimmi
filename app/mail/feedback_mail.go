/*
|--------------------------------------------------------------------------
| Feature Request Mail
|--------------------------------------------------------------------------
|
| When a feature request moves, the people behind it hear once: who
| voted for it and who posted it. The mail says where it stands, carries
| the team's note, and links the update that shipped it.
|
*/

package mail

import (
	"context"

	"github.com/dimmihq/dimmi/framework/i18n"
	"github.com/dimmihq/dimmi/framework/mail"

	"github.com/dimmihq/dimmi/resources/views/mail/feedback"
)

/*
FeedbackStatusMail tells one person a request they backed moved.
*/
type FeedbackStatusMail struct {
	Moved feedback.Moved `json:"moved"`
}

/*
Envelope tags the mail as a feature request update.
*/
func (FeedbackStatusMail) Envelope() mail.Envelope {
	return mail.Envelope{Tags: map[string]string{"kind": "feedback_status"}}
}

/*
Content renders the mail.
*/
func (m FeedbackStatusMail) Content(ctx context.Context) (mail.Content, error) {
	c := mail.View(feedback.StatusChanged(m.Moved))
	c.Subject = i18n.T(ctx, "mail.feedback_status.subject", i18n.Args{"title": m.Moved.Title, "status": m.Moved.Status})
	return c, nil
}
