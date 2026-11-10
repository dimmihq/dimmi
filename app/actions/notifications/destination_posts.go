/*
|--------------------------------------------------------------------------
| Posts To Slack Channels
|--------------------------------------------------------------------------
|
| A Slack channel the team picked hears about the triggers it listed,
| from the inboxes it listed or from all of them. It keeps a log, so it
| hears even when a member acted or nobody is told; only conversations
| the agent is still answering stay quiet unless the channel asked for
| them, and a handoff always posts. Previews follow the channel's own
| setting, never a member's.
|
*/

package notifications

import (
	"context"
	"fmt"
	"slices"

	"github.com/dimmihq/dimmi/app/models"
)

/*
post queues the event's post to every Slack channel that listed it.
*/
func (d *Dispatcher) post(ctx context.Context, rd round) error {
	if !slices.Contains(DestinationTriggers(), rd.ev.Trigger) {
		return nil
	}

	rows, err := models.New(rd.tx).ListActiveChannelDestinations(ctx, rd.ev.WorkspaceID)
	if err != nil {
		return fmt.Errorf("load notification destinations: %w", err)
	}

	for _, row := range rows {
		if !hears(row, rd.ev, rd.subject) {
			continue
		}

		settings := DefaultSettings(rd.ev.WorkspaceID, rd.ev.WorkspaceID)
		settings.ShowPreviews = row.ShowPreviews
		a := rd.subject.alert(rd.ev, Choices{Settings: settings}, d.Links)
		a.checker = d.Checker

		if err := d.Notify.SendTx(ctx, rd.tx, withChannels(a, SlackChannelName), Destination{Row: row}); err != nil {
			return fmt.Errorf("queue slack post: %w", err)
		}
	}

	return nil
}

/*
hears reports whether the channel listed the event.
*/
func hears(row models.NotificationDestination, ev Event, s *subject) bool {
	if !slices.Contains(row.Triggers, ev.Trigger) {
		return false
	}

	if s.aiBusy(ev) && !row.IncludeAiHandled {
		return false
	}

	if row.InboxIds == nil || s.conversation == nil {
		return true
	}

	return slices.Contains(row.InboxIds, s.conversation.InboxID)
}
