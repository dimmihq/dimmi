/*
|--------------------------------------------------------------------------
| Changing Retention
|--------------------------------------------------------------------------
|
| Each period has a range a workspace may choose within: long enough that
| nothing disappears by accident, short enough to mean something. Raw
| email may go at once, 0. Closed conversations are kept for ever until
| a period is set, and setting null keeps them again.
|
*/

package workspaces

import (
	"time"

	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
RetentionChange is a change to the retention periods; nil leaves one as
it is, and ClearClosedConversationDays keeps closed conversations for
ever again.
*/
type RetentionChange struct {
	ClosedConversationDays      *int
	ClearClosedConversationDays bool
	SpamDays                    *int
	TrashDays                   *int
	RawEmailDays                *int
	ExportDays                  *int
}

/*
period is one retention period: where its value goes and the range it
must fall in.
*/
type period struct {
	field    string
	sent     *int
	target   *int
	min, max int
}

/*
applyRetention writes the change into the settings and reports whether
anything changed.
*/
func applyRetention(
	params *models.UpdateWorkspaceParams, w models.Workspace, change *RetentionChange, now time.Time,
) (bool, error) {
	if change == nil {
		return false, nil
	}

	current := w.Retention()
	next := current

	if err := closedDays(&next, change); err != nil {
		return false, err
	}

	for _, p := range []period{
		{"spam_days", change.SpamDays, &next.SpamDays, 7, 365},
		{"trash_days", change.TrashDays, &next.TrashDays, 7, 365},
		{"raw_email_days", change.RawEmailDays, &next.RawEmailDays, 0, 365},
		{"export_days", change.ExportDays, &next.ExportDays, 1, 30},
	} {
		if p.sent == nil {
			continue
		}

		if *p.sent < p.min || *p.sent > p.max {
			return false, exceptions.ErrInvalidRetention.With("field", p.field)
		}

		*p.target = *p.sent
	}

	if sameRetention(current, next) {
		return false, nil
	}

	next.ChangedAt = &now

	var err error

	params.Settings, err = models.WithArea(params.Settings, "retention", next)

	return err == nil, err
}

/*
closedDays applies the change to the closed conversation period.
*/
func closedDays(next *models.Retention, change *RetentionChange) error {
	if change.ClearClosedConversationDays {
		next.ClosedConversationDays = nil

		return nil
	}

	if change.ClosedConversationDays == nil {
		return nil
	}

	days := *change.ClosedConversationDays
	if days < 30 || days > 3650 {
		return exceptions.ErrInvalidRetention.With("field", "closed_conversation_days")
	}

	next.ClosedConversationDays = &days

	return nil
}

/*
sameRetention reports whether two sets of periods are the same.
*/
func sameRetention(a, b models.Retention) bool {
	sameClosed := (a.ClosedConversationDays == nil) == (b.ClosedConversationDays == nil) &&
		(a.ClosedConversationDays == nil || *a.ClosedConversationDays == *b.ClosedConversationDays)

	return sameClosed && a.SpamDays == b.SpamDays && a.TrashDays == b.TrashDays &&
		a.RawEmailDays == b.RawEmailDays && a.ExportDays == b.ExportDays
}
