/*
|--------------------------------------------------------------------------
| Quiet Hours Summary
|--------------------------------------------------------------------------
|
| Email that quiet hours held back is not lost: once a teammate's quiet
| hours are over, one email lists what came in meanwhile. The delivery
| log is the list: every email skipped for quiet hours waits there
| until a summary covers it. A member still in quiet hours, or paused,
| waits for the next sweep. Held pushes are covered too, so the next
| summary does not list them again, but only held emails are listed,
| since a member who only wanted a push did not ask for mail.
|
*/

package notifications

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/i18n"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/mail"
	fnotify "github.com/dimmihq/dimmi/framework/notifications"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/resources/views/mail/accounts"
)

/*
QuietSummaryType names the summary among notifications.
*/
const QuietSummaryType = "quiet_hours_summary"

/*
Summary limits: how many members one sweep reads and how many lines
one email lists.
*/
const (
	summaryBatch = 200
	summaryLines = 10
)

/*
QuietSummaries sends the summaries that are due.
*/
type QuietSummaries struct {
	DB         *database.DB
	Clock      clock.Clock
	Notify     *fnotify.Manager
	Recipients *Recipients
	Links      Links
}

/*
Due sends a summary to every member whose quiet hours held email back
and are now over.
*/
func (s *QuietSummaries) Due(ctx context.Context) error {
	rows, err := models.New(s.DB).ListQuietHeldMembers(ctx, summaryBatch)
	if err != nil {
		return fmt.Errorf("list members with held notifications: %w", err)
	}
	var errs []error
	for _, row := range rows {
		errs = append(errs, s.Send(ctx, row.WorkspaceID, row.WorkspaceMemberID))
	}
	return errors.Join(errs...)
}

/*
Send sends one member their summary if their quiet hours are over, and
marks what it covered.
*/
func (s *QuietSummaries) Send(ctx context.Context, ws, member ids.UUID) error {
	var (
		summary *QuietSummary
		to      Recipient
	)
	err := s.DB.Transaction(tenancy.With(ctx, ws.String()), func(ctx context.Context, tx *database.Tx) error {
		var err error
		summary, to, err = s.collect(ctx, models.New(tx), ws, member)
		return err
	})
	if err != nil || summary == nil {
		return err
	}
	return s.Notify.Send(ctx, summary, to)
}

/*
collect marks what quiet hours held back from the member once they are
over, and writes the summary when any of it was email to an address
that takes it.
*/
func (s *QuietSummaries) collect(ctx context.Context, q *models.Queries, ws, member ids.UUID,
) (*QuietSummary, Recipient, error) {
	found, err := s.Recipients.Load(ctx, q, ws, []ids.UUID{member})
	if err != nil {
		return nil, Recipient{}, err
	}
	choices, err := LoadChoices(ctx, q, ws, member)
	if err != nil {
		return nil, Recipient{}, err
	}
	if len(found) > 0 && choices.QuietReason(s.Clock.Now(), "", found[0].Timezone) != "" {
		return nil, Recipient{}, nil
	}
	held, err := q.ListQuietHeldDeliveries(ctx, models.ListQuietHeldDeliveriesParams{WorkspaceID: ws, MemberID: member})
	if err != nil {
		return nil, Recipient{}, fmt.Errorf("list held notifications: %w", err)
	}
	if err := q.MarkQuietDeliveriesSummarized(ctx, models.MarkQuietDeliveriesSummarizedParams{
		Now: s.Clock.Now(), WorkspaceID: ws, MemberID: member,
	}); err != nil {
		return nil, Recipient{}, fmt.Errorf("mark held notifications: %w", err)
	}
	if len(found) == 0 || !found[0].EmailVerified {
		return nil, Recipient{}, nil
	}
	summary, err := s.summary(ctx, q, found[0], emailed(held))
	return summary, found[0], err
}

/*
emailed keeps the held lines that were emails.
*/
func emailed(held []models.NotificationDelivery) []models.NotificationDelivery {
	var out []models.NotificationDelivery
	for _, h := range held {
		if h.Channel == ChannelEmail {
			out = append(out, h)
		}
	}
	return out
}

/*
summary writes the member's summary, nil when nothing was emailed.
*/
func (s *QuietSummaries) summary(ctx context.Context, q *models.Queries, to Recipient,
	held []models.NotificationDelivery,
) (*QuietSummary, error) {
	if len(held) == 0 {
		return nil, nil
	}
	w, err := q.FindWorkspace(ctx, to.WorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("load workspace: %w", err)
	}
	out := &QuietSummary{
		Workspace: w.Name, URL: s.Links.Home(w.Slug), ManageURL: s.Links.Settings(w.Slug),
		Unsubscribe: s.Links.Unsubscribe(to.WorkspaceID, to.MemberID, AllTriggers, s.Clock.Now()),
	}
	for i, h := range held {
		if i >= summaryLines {
			out.More = len(held) - summaryLines
			break
		}
		out.Lines = append(out.Lines, accounts.QuietLine{Title: h.Title, URL: h.Link})
	}
	return out, nil
}

/*
QuietSummary is the email listing what quiet hours held back.
*/
type QuietSummary struct {
	Workspace   string               `json:"workspace"`
	Lines       []accounts.QuietLine `json:"lines"`
	More        int                  `json:"more"`
	URL         string               `json:"url"`
	ManageURL   string               `json:"manage_url"`
	Unsubscribe string               `json:"unsubscribe,omitempty"`
}

/*
Type names the notification.
*/
func (*QuietSummary) Type() string { return QuietSummaryType }

/*
Via sends the summary by mail only.
*/
func (*QuietSummary) Via(fnotify.Notifiable) []string { return []string{"mail"} }

/*
ToMail is the email.
*/
func (s *QuietSummary) ToMail(context.Context, fnotify.Delivery) (mail.Mailable, error) {
	return QuietSummaryMail{
		Workspace: s.Workspace, Lines: s.Lines, More: s.More, URL: s.URL, ManageURL: s.ManageURL,
		Unsubscribe: s.Unsubscribe,
	}, nil
}

/*
QuietSummaryMail is the summary as an email.
*/
type QuietSummaryMail struct {
	Workspace   string               `json:"workspace"`
	Lines       []accounts.QuietLine `json:"lines"`
	More        int                  `json:"more"`
	URL         string               `json:"url"`
	ManageURL   string               `json:"manage_url"`
	Unsubscribe string               `json:"unsubscribe,omitempty"`
}

/*
Envelope offers the one-click unsubscribe.
*/
func (m QuietSummaryMail) Envelope() mail.Envelope { return unsubscribeEnvelope(m.Unsubscribe) }

/*
Content renders the summary.
*/
func (m QuietSummaryMail) Content(ctx context.Context) (mail.Content, error) {
	subject := i18n.T(ctx, "mail.quiet_summary.subject", i18n.Args{"workspace": m.Workspace})
	c := mail.View(accounts.QuietHoursSummary(subject, m.Workspace, m.Lines, m.More, m.URL, m.ManageURL,
		m.Unsubscribe))
	c.Subject = subject
	return c, nil
}
