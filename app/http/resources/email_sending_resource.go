/*
|--------------------------------------------------------------------------
| Email Sending Resource
|--------------------------------------------------------------------------
|
| A workspace's standing as a sender, as the settings page shows it:
| whether bulk email is paused and why, what the last week of sending
| looked like, and the rates past which it pauses.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/app/actions/email"
)

/*
EmailSending is the workspace's standing as a sender. The rates are
shares between 0 and 1, zero when the install never pauses for them.
*/
type EmailSending struct {
	Object             string     `json:"object"`
	Status             string     `json:"status"              enum:"active,paused"`
	PausedAt           *time.Time `json:"paused_at"`
	PausedReason       *string    `json:"paused_reason"       enum:"bounce_rate,complaint_rate"`
	Sent               int64      `json:"sent"`
	HardBounced        int64      `json:"hard_bounced"`
	Complained         int64      `json:"complained"`
	WindowDays         int        `json:"window_days"`
	MinimumSent        int        `json:"minimum_sent"`
	BouncePauseRate    float64    `json:"bounce_pause_rate"`
	ComplaintPauseRate float64    `json:"complaint_pause_rate"`
}

/*
NewEmailSending shapes a workspace's standing as a sender.
*/
func NewEmailSending(s email.SendingStatus) EmailSending {
	out := EmailSending{
		Object: "email_sending", Status: "active", Sent: s.Counts.Sent, HardBounced: s.Counts.HardBounced,
		Complained: s.Counts.Complained, WindowDays: int(email.ReputationWindow.Hours() / 24),
		MinimumSent:     email.ReputationMinimum,
		BouncePauseRate: s.Rates.BouncePauseRate, ComplaintPauseRate: s.Rates.ComplaintPauseRate,
	}

	if s.Paused {
		out.Status, out.PausedAt = "paused", s.PausedAt
		if s.Reason != "" {
			out.PausedReason = &s.Reason
		}
	}

	return out
}
