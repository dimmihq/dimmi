/*
|--------------------------------------------------------------------------
| Views Opened At Once
|--------------------------------------------------------------------------
|
| Slack lets a click open a modal only for three seconds after it, so a
| button that opens one, such as editing an agent's draft, is answered
| while Slack waits for the request instead of from the queue, where a
| busy moment would let the click expire and do nothing. Should opening
| it fail, the click is kept like any other and tried from the queue.
|
*/

package channels

import (
	"context"

	slackdriver "github.com/dimmihq/dimmi/app/drivers/channel/slack"
	"github.com/dimmihq/dimmi/app/models"
)

/*
openedNow opens the modal a click asks for while the click is fresh,
and reports whether it did, so the click needs nothing more.
*/
func (s *Slack) openedNow(ctx context.Context, r routed, in slackdriver.Interaction) bool {
	action, ok := viewOpener(in)
	if !ok || r.suspended {
		return false
	}

	conn, live, err := s.ownerConnection(ctx, models.ChannelInboundEvent{
		WorkspaceID: r.workspaceID, ChannelConnectionID: connectionOf(r), IntegrationInstallationID: r.installationID,
	})
	if err != nil || !live {
		return false
	}

	handled, err := s.onDraftAction(ctx, SlackInteraction{
		Connection: conn, WorkspaceID: conn.WorkspaceID, InstallationID: conn.IntegrationInstallationID,
		Interaction: in, Action: action,
	})

	return handled && err == nil
}

/*
viewOpener returns the click's only action when it opens a modal.
*/
func viewOpener(in slackdriver.Interaction) (slackdriver.BlockAction, bool) {
	if in.Type != slackdriver.InteractionBlockActions || len(in.Actions) != 1 || in.TriggerID == "" {
		return slackdriver.BlockAction{}, false
	}

	_, verb, ok := slackdriver.ParseTeamActionID(slackdriver.DraftPrefix(), in.Actions[0].ActionID)

	return in.Actions[0], ok && verb == slackdriver.DraftEdit
}
