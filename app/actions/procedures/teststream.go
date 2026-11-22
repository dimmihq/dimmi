/*
|--------------------------------------------------------------------------
| Watching A Simulation Run
|--------------------------------------------------------------------------
|
| A saved simulation replays several messages against the real agent,
| which takes a while. The teammate who started it watches it happen:
| each customer message as it is sent, what dimmi does while it works,
| each reply as it lands, then the checks being judged and the verdict.
| The events go only to that teammate, are never stored, and carry the
| stream id their browser chose, so a second run cannot be mistaken for
| the first. The saved result is still what the request returns.
|
*/

package procedures

import (
	"context"
	"maps"
	"strconv"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Live sends an event of a simulation run to one teammate's browser.
*/
type Live interface {
	/*
		PublishRun sends the event to the member, or nowhere when live
		updates are not set up.
	*/
	PublishRun(ctx context.Context, member ids.UUID, data map[string]any)
}

/*
TestRun is what to run: the tests named, every one when none is, and
the stream the teammate's browser listens on, empty to run unwatched.
*/
type TestRun struct {
	Only     []ids.UUID
	StreamID string
}

/*
The moments of a run a watcher is told about.
*/
const (
	runStarted      = "started"
	runTurnStarted  = "turn_started"
	runTurnFinished = "turn_finished"
	runJudging      = "judging"
	runFinished     = "finished"
)

/*
watch is one test's run as its watcher sees it.
*/
type watch struct {
	p      *Procedures
	member ids.UUID
	stream string
	test   string
}

/*
watching opens the watch of one test's run, silent when nobody watches.
*/
func (p *Procedures) watching(m authz.Membership, run TestRun, c models.ProcedureTestCase) watch {
	return watch{
		p: p, member: m.Member.ID, stream: run.StreamID, test: models.PublicID(models.PrefixProcedureTest, c.ID),
	}
}

/*
tell sends one moment of the run with its details.
*/
func (w watch) tell(ctx context.Context, moment string, details map[string]any) {
	if w.stream == "" || w.p.Live == nil {
		return
	}
	data := map[string]any{"stream_id": w.stream, "test_case_id": w.test, "moment": moment}
	maps.Copy(data, details)
	w.p.Live.PublishRun(ctx, w.member, data)
}

/*
turnStream names the stream of one turn's steps, which the agent's own
tool calls are published on, or nothing when nobody watches.
*/
func (w watch) turnStream(turn int) string {
	if w.stream == "" {
		return ""
	}
	return w.stream + ":" + w.test + ":" + strconv.Itoa(turn)
}
