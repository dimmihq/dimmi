/*
|--------------------------------------------------------------------------
| Slack Connect Routing
|--------------------------------------------------------------------------
|
| When two organizations that both use dimmi share a channel, Slack may
| deliver each message once, for one of them, and lists only some of
| the other installations that can see it. A message in a shared
| channel is therefore also routed to every installation of the shared
| app that watches the channel, so each workspace keeps its own copy
| of the conversation and no customer message is lost on either side.
|
*/

package channels

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"

	slackdriver "github.com/dimmihq/dimmi/app/drivers/channel/slack"
	"github.com/dimmihq/dimmi/app/models"
)

/*
teamsFor returns the teams an event is for: those Slack named, and in
a channel shared with another organization every team on the shared
app watching that channel.
*/
func (s *Slack) teamsFor(ctx context.Context, own *routed, env slackdriver.Envelope, ev slackdriver.Event,
) ([]string, error) {
	teams := teamsOf(env)
	if own != nil || s.Shared == nil || !env.IsExtSharedChannel || ev.ChannelID() == "" {
		return teams, nil
	}

	keys, err := models.New(s.DB).RouteRoomConnections(ctx, models.RouteRoomConnectionsParams{
		ConnectionType: slackdriver.Type, RoomExternalID: ev.ChannelID(),
	})
	if err != nil {
		return nil, fmt.Errorf("route shared slack channel: %w", err)
	}

	prefix := slackdriver.ExternalID(cmp.Or(env.APIAppID, s.Shared.AppID), "")

	for _, key := range keys {
		team, ok := strings.CutPrefix(key, prefix)
		if ok && team != "" && !slices.Contains(teams, team) {
			teams = append(teams, team)
		}
	}

	return teams, nil
}
