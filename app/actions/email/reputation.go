/*
|--------------------------------------------------------------------------
| Sending Reputation
|--------------------------------------------------------------------------
|
| A workspace whose mail bounces or is reported as spam too often harms
| the addresses it sends from, and on shared infrastructure everyone
| else's too. Every customer email a workspace sends is counted by day,
| and so is every one that bounced for good or was reported as spam.
| Once enough was sent in the last week to judge by, a workspace past
| either rate has its sending paused: broadcasts, series and rating
| emails wait, while replies to customers keep going, so support never
| goes dark. Its owners and admins are mailed, the audit log says why,
| and an owner resumes it, after which the count starts again.
|
| A rate of zero is never judged, which is how an install without the
| rates set never pauses.
|
*/

package email

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/mail"

	"github.com/dimmihq/dimmi/app/actions/audit"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/config"
)

/*
Why a workspace's sending paused, how many days of sending it is judged
by, and how many emails those days must hold before it is judged.
*/
const (
	PausedBounceRate    = "bounce_rate"
	PausedComplaintRate = "complaint_rate"
	ReputationWindow    = 7 * 24 * time.Hour
	ReputationMinimum   = 100
)

/*
SendingCounts is what a workspace sent to customers in the window and
how it fared.
*/
type SendingCounts struct {
	Sent        int64
	HardBounced int64
	Complained  int64
}

/*
Harmful returns why counts call for a pause, or nothing. Fewer than
ReputationMinimum emails are too few to judge by, a rate of zero is not
judged, and complaints are looked at before bounces.
*/
func Harmful(c SendingCounts, rates config.EmailReputation) string {
	if c.Sent < ReputationMinimum {
		return ""
	}

	sent := float64(c.Sent)

	switch {
	case rates.ComplaintPauseRate > 0 && float64(c.Complained)/sent > rates.ComplaintPauseRate:
		return PausedComplaintRate
	case rates.BouncePauseRate > 0 && float64(c.HardBounced)/sent > rates.BouncePauseRate:
		return PausedBounceRate
	}

	return ""
}

/*
SendingLinks finds the address of the application's pages.
*/
type SendingLinks interface {
	/*
		App returns the address the application is served at.
	*/
	App() string
}

/*
SendingStatus is a workspace's standing as a sender: whether it is
paused and why, what it is judged by, and the rates that pause it.
*/
type SendingStatus struct {
	Paused   bool
	PausedAt *time.Time
	Reason   string
	Counts   SendingCounts
	Rates    config.EmailReputation
}

/*
Reputation counts what workspaces send and how it fares, pauses the
ones past a rate, and lets an owner resume them.

Rules:
  - Reading the status needs settings:read.
  - Resuming needs email:resume_sending. Resuming a workspace that is
    not paused changes nothing.

Errors: [exceptions.ErrForbidden], or a wrapped error.
*/
type Reputation struct {
	DB    *database.DB
	Clock clock.Clock
	Mail  *mail.Manager
	Links SendingLinks
	Audit *audit.RecordAuditLog
	Rates config.EmailReputation
}

/*
Sent counts one customer email the workspace sent.
*/
func (r *Reputation) Sent(ctx context.Context, q *models.Queries, workspaceID ids.UUID) error {
	if r == nil {
		return nil
	}

	err := q.CountEmailSent(ctx, models.CountEmailSentParams{WorkspaceID: workspaceID, Now: r.Clock.Now()})
	if err != nil {
		return fmt.Errorf("count sent email: %w", err)
	}

	return nil
}

/*
Reported counts one customer email that bounced for good, or one that
was reported as spam.
*/
func (r *Reputation) Reported(ctx context.Context, q *models.Queries, workspaceID ids.UUID, complaint bool) error {
	if r == nil {
		return nil
	}

	now := r.Clock.Now()

	var err error
	if complaint {
		err = q.CountEmailComplaint(ctx, models.CountEmailComplaintParams{WorkspaceID: workspaceID, Now: now})
	} else {
		err = q.CountEmailHardBounce(ctx, models.CountEmailHardBounceParams{WorkspaceID: workspaceID, Now: now})
	}

	if err != nil {
		return fmt.Errorf("count email report: %w", err)
	}

	return nil
}

/*
Judge pauses the workspace's sending when its last week is past a rate,
and reports whether it paused it now. A workspace paused already is
left as it is.
*/
func (r *Reputation) Judge(ctx context.Context, workspaceID ids.UUID) (bool, error) {
	if r == nil || (r.Rates.BouncePauseRate <= 0 && r.Rates.ComplaintPauseRate <= 0) {
		return false, nil
	}

	paused := false

	err := r.DB.Transaction(inWorkspace(ctx, workspaceID), func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		counts, err := r.counts(ctx, q, workspaceID)
		if err != nil {
			return err
		}

		reason := Harmful(counts, r.Rates)
		if reason == "" {
			return nil
		}

		now := r.Clock.Now()

		_, err = q.PauseEmailSending(ctx, models.PauseEmailSendingParams{
			WorkspaceID: workspaceID, Reason: &reason, Now: now,
		})
		if errors.Is(err, database.ErrNotFound) {
			return nil
		}

		if err != nil {
			return fmt.Errorf("pause email sending: %w", err)
		}

		paused = true

		if _, err := q.PauseSendingEmailBroadcasts(ctx, models.PauseSendingEmailBroadcastsParams{
			WorkspaceID: workspaceID, Reason: new(BlockedSendingPaused), Now: now,
		}); err != nil {
			return fmt.Errorf("pause email broadcasts: %w", err)
		}

		return r.tell(ctx, tx, workspaceID, reason, counts)
	})

	return paused, err
}

/*
counts adds up the workspace's last week of sending.
*/
func (r *Reputation) counts(ctx context.Context, q *models.Queries, workspaceID ids.UUID) (SendingCounts, error) {
	row, err := q.SumEmailSendingStats(ctx, models.SumEmailSendingStatsParams{
		WorkspaceID: workspaceID, Since: r.Clock.Now().Add(-ReputationWindow + 24*time.Hour),
	})
	if err != nil {
		return SendingCounts{}, fmt.Errorf("sum sending stats: %w", err)
	}

	return SendingCounts{Sent: row.Sent, HardBounced: row.HardBounced, Complained: row.Complained}, nil
}

/*
tell writes the pause to the audit log and mails the workspace's owners
and admins why sending paused and where to resume it.
*/
func (r *Reputation) tell(
	ctx context.Context, tx *database.Tx, workspaceID ids.UUID, reason string, counts SendingCounts,
) error {
	q := models.New(tx)

	workspace, err := q.FindWorkspace(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("find workspace to tell: %w", err)
	}

	if r.Audit != nil {
		if _, err := r.Audit.Handle(ctx, audit.Entry{
			WorkspaceID: workspaceID, Event: audit.EventEmailSendingPaused,
			Subject: audit.Subject{Type: "workspace", ID: workspaceID, Label: workspace.Name},
			Changes: map[string]any{
				"reason": reason, "sent": counts.Sent, "hard_bounced": counts.HardBounced,
				"complained": counts.Complained,
			},
		}); err != nil {
			return fmt.Errorf("record sending pause: %w", err)
		}
	}

	if r.Mail == nil {
		return nil
	}

	people, err := q.ListSendingAlertRecipients(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("list sending alert recipients: %w", err)
	}

	for _, p := range people {
		m := appmail.EmailSendingPausedMail{
			Name: p.User.Name, Workspace: workspace.Name, Reason: reason, Sent: counts.Sent,
			HardBounced: counts.HardBounced, Complained: counts.Complained,
			SettingsURL: r.Links.App() + "/settings/email",
		}
		if err := appmail.ToUser(r.Mail, p.User).QueueTx(ctx, tx, m); err != nil {
			return fmt.Errorf("queue sending paused mail: %w", err)
		}
	}

	return nil
}

/*
Status returns the workspace's standing as a sender.
*/
func (r *Reputation) Status(ctx context.Context, m authz.Membership) (SendingStatus, error) {
	if !m.Can("settings:read") {
		return SendingStatus{}, exceptions.ErrForbidden
	}

	out := SendingStatus{Rates: r.Rates}

	err := r.DB.Transaction(inWorkspace(ctx, m.Workspace.ID), func(ctx context.Context, tx *database.Tx) error {
		var err error

		out, err = r.status(ctx, models.New(tx), m.Workspace.ID)

		return err
	})

	return out, err
}

/*
status reads the standing inside a transaction.
*/
func (r *Reputation) status(ctx context.Context, q *models.Queries, workspaceID ids.UUID) (SendingStatus, error) {
	out := SendingStatus{Rates: r.Rates}

	profile, err := q.FindEmailSendingProfile(ctx, workspaceID)
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return out, fmt.Errorf("find email sending profile: %w", err)
	}

	if err == nil && profile.Status == "paused" {
		out.Paused, out.PausedAt = true, profile.PausedAt
		if profile.PausedReason != nil {
			out.Reason = *profile.PausedReason
		}
	}

	out.Counts, err = r.counts(ctx, q, workspaceID)

	return out, err
}

/*
Resume lets a paused workspace send again and forgets its counts, so it
is judged on what it sends from now on. Broadcasts the pause held stay
paused until a teammate resumes each.
*/
func (r *Reputation) Resume(ctx context.Context, m authz.Membership) (SendingStatus, error) {
	if !m.Can("email:resume_sending") {
		return SendingStatus{}, exceptions.ErrForbidden
	}

	var out SendingStatus

	ws := m.Workspace.ID

	err := r.DB.Transaction(inWorkspace(ctx, ws), func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		_, err := q.ResumeEmailSending(ctx, models.ResumeEmailSendingParams{WorkspaceID: ws, Now: r.Clock.Now()})
		if errors.Is(err, database.ErrNotFound) {
			out, err = r.status(ctx, q, ws)

			return err
		}

		if err != nil {
			return fmt.Errorf("resume email sending: %w", err)
		}

		if _, err := q.ResetEmailSendingStats(ctx, ws); err != nil {
			return fmt.Errorf("reset sending stats: %w", err)
		}

		if r.Audit != nil {
			if _, err := r.Audit.Handle(ctx, audit.Entry{
				WorkspaceID: ws, Event: audit.EventEmailSendingResumed, ActorID: m.Member.ID,
				Subject: audit.Subject{Type: "workspace", ID: ws, Label: m.Workspace.Name},
			}); err != nil {
				return fmt.Errorf("record sending resume: %w", err)
			}
		}

		out, err = r.status(ctx, q, ws)

		return err
	})

	return out, err
}
