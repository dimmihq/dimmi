/*
|--------------------------------------------------------------------------
| Join Slack Channels
|--------------------------------------------------------------------------
|
| A connection's auto-join rules are checked against every channel of
| its team, a page at a time, each page its own job, so a long scan
| stays within Slack's pace and resumes where it stopped.
|
*/

package jobs

import (
	"context"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"
)

/*
JoinSlackChannelsArgs names a connection whose public channels are
checked against its auto-join rules, from the page the cursor names.
*/
type JoinSlackChannelsArgs struct {
	WorkspaceID  ids.UUID `json:"workspace_id"`
	ConnectionID ids.UUID `json:"connection_id"`
	Cursor       string   `json:"cursor,omitempty"`
}

/*
Kind names the job.
*/
func (JoinSlackChannelsArgs) Kind() string {
	return "join_slack_channels"
}

/*
joinAttempts is how often a page is tried.
*/
const joinAttempts = 5

/*
Options try each page a few times.
*/
func (JoinSlackChannelsArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: joinAttempts}
}

/*
ChannelJoiner joins the channels a connection's rules match.
*/
type ChannelJoiner interface {
	/*
		JoinChannels checks one page of channels, queueing the next.
	*/
	JoinChannels(ctx context.Context, args JoinSlackChannelsArgs) error
}

/*
JoinSlackChannels runs the auto-join scans.
*/
type JoinSlackChannels struct {
	Joiner ChannelJoiner
}

/*
Handle checks the page.
*/
func (w *JoinSlackChannels) Handle(ctx context.Context, job *queue.Job[JoinSlackChannelsArgs]) error {
	return w.Joiner.JoinChannels(ctx, job.Args)
}
