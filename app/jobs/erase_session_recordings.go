/*
|--------------------------------------------------------------------------
| Erase Session Recordings
|--------------------------------------------------------------------------
|
| The dashboard records its teammates' sessions on a kimmi recorder and
| names each teammate there as "<workspace>:<member>", by the public ids
| the dashboard knows. A workspace purged for good takes its recordings
| with it: this job asks the recorder to erase every visitor whose
| external id starts with the workspace's public id and a colon.
|
| The purge queues it with the workspace's last delete, so a recorder that
| is down never holds up or fails the purge; the job retries with backoff
| and is reported once its last attempt fails. Erasing twice is harmless:
| the second call finds no one left.
|
*/

package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/log"
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app/models"
)

/*
Limits on erasing recordings: how long one call may take, how long one
run may take, and how often a failing erase is tried. River's backoff
spreads the attempts over most of a day.
*/
const (
	eraseRecordingsRequest  = 15 * time.Second
	eraseRecordingsRun      = 30 * time.Second
	eraseRecordingsAttempts = 12
)

/*
EraseSessionRecordingsArgs names the purged workspace whose recordings
go.
*/
type EraseSessionRecordingsArgs struct {
	WorkspaceID ids.UUID `json:"workspace_id"`
}

/*
Kind names the job.
*/
func (EraseSessionRecordingsArgs) Kind() string {
	return "erase_session_recordings"
}

/*
Options run one erase per workspace at a time, retried for most of a day
when the recorder fails.
*/
func (EraseSessionRecordingsArgs) Options() queue.Options {
	return queue.Options{
		MaxAttempts: eraseRecordingsAttempts,
		Unique: queue.UniqueOpts{ByArgs: true, ByState: []queue.State{
			queue.StateAvailable, queue.StatePending, queue.StateRetryable, queue.StateRunning, queue.StateScheduled,
		}},
	}
}

/*
SessionRecordingsPrefix returns the start every recorder external id of
the workspace's teammates shares: its public id and a colon, as the
dashboard names them.
*/
func SessionRecordingsPrefix(workspaceID ids.UUID) string {
	return models.PublicID(models.PrefixWorkspace, workspaceID) + ":"
}

/*
EraseSessionRecordings erases a workspace's recordings on the recorder.
URL is the recorder's erase address and Token its API token; with either
empty the job sends nothing.
*/
type EraseSessionRecordings struct {
	HTTP  *httpclient.Client
	URL   string
	Token string
	Log   log.Logger
}

/*
Timeout bounds one run.
*/
func (*EraseSessionRecordings) Timeout() time.Duration { return eraseRecordingsRun }

/*
Handle asks the recorder to erase the workspace's visitors and their
recordings. Any answer but a 2xx fails the run, so it is tried again.
*/
func (w *EraseSessionRecordings) Handle(ctx context.Context, job *queue.Job[EraseSessionRecordingsArgs]) error {
	if w.URL == "" || w.Token == "" {
		return nil
	}

	workspace := job.Args.WorkspaceID.String()
	body := map[string]string{"externalIdPrefix": SessionRecordingsPrefix(job.Args.WorkspaceID)}

	res, err := w.HTTP.New().Timeout(eraseRecordingsRequest).WithToken(w.Token).WithoutRedirects().
		Post(ctx, w.URL, body)
	if err != nil {
		w.Log.WarnContext(ctx, "session recordings erase failed", log.String("workspace_id", workspace),
			log.Int("attempt", job.Attempt), log.Err(err))

		return fmt.Errorf("erase session recordings: %w", err)
	}

	if res.Status < 200 || res.Status > 299 {
		w.Log.WarnContext(ctx, "session recordings erase refused", log.String("workspace_id", workspace),
			log.Int("attempt", job.Attempt), log.Int("status", res.Status))

		return fmt.Errorf("erase session recordings: recorder answered %d", res.Status)
	}

	var out struct {
		Deleted int `json:"deleted"`
	}

	_ = res.JSON(&out)

	w.Log.InfoContext(ctx, "session recordings erased", log.String("workspace_id", workspace),
		log.Int("visitors", out.Deleted))

	return nil
}
