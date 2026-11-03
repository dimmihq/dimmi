/*
|--------------------------------------------------------------------------
| Purge Expired Auth Records
|--------------------------------------------------------------------------
|
| Signing in leaves records behind that stop meaning anything once they
| run out: authenticator enrollments started and never finished, mailed
| links and codes that expired or were used, password resets past their
| time, and sessions nobody can resume. Every hour they are deleted, so
| they neither pile up nor linger as secrets nobody needs.
|
| Each record already stops working by its own timestamps; deleting it
| changes nothing anyone can do. Spent one-time tokens are kept for a day
| first, so a person opening a link twice still learns it was used.
|
*/

package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app/models"
)

/*
SpentTokenRetention is how long a one-time token is kept once it
expired or was used, and purgeAttempts how often the job is tried.
*/
const (
	SpentTokenRetention = 24 * time.Hour
	purgeAttempts       = 3
)

/*
PurgeExpiredAuthRecordsArgs starts one purge.
*/
type PurgeExpiredAuthRecordsArgs struct{}

/*
Kind names the job.
*/
func (PurgeExpiredAuthRecordsArgs) Kind() string {
	return "purge_expired_auth_records"
}

/*
Options try the job a few times and run it once per hour however many
times it is pushed: the next run catches up on anything left.
*/
func (PurgeExpiredAuthRecordsArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: purgeAttempts, Unique: queue.UniqueOpts{ByPeriod: time.Hour}}
}

/*
PurgeExpiredAuthRecords deletes the sign-in records that ran out.
PendingFactor is how long an enrollment may stay unfinished and
SessionIdle how long an unused session lasts.
*/
type PurgeExpiredAuthRecords struct {
	DB            *database.DB
	Clock         clock.Clock
	PendingFactor time.Duration
	SessionIdle   time.Duration
}

/*
Handle deletes every kind of expired record.
*/
func (w *PurgeExpiredAuthRecords) Handle(ctx context.Context, _ *queue.Job[PurgeExpiredAuthRecordsArgs]) error {
	q := models.New(w.DB)
	now := w.Clock.Now()

	if _, err := q.DeletePendingAuthFactorsBefore(ctx, now.Add(-w.PendingFactor)); err != nil {
		return fmt.Errorf("purge unfinished factors: %w", err)
	}

	if _, err := q.DeleteSpentOneTimeTokensBefore(ctx, now.Add(-SpentTokenRetention)); err != nil {
		return fmt.Errorf("purge spent one-time tokens: %w", err)
	}

	if _, err := q.DeleteExpiredPasswordResets(ctx, now); err != nil {
		return fmt.Errorf("purge expired password resets: %w", err)
	}

	_, err := q.DeleteExpiredSessions(ctx, models.DeleteExpiredSessionsParams{
		Now: now, IdleBefore: now.Add(-w.SessionIdle),
	})
	if err != nil {
		return fmt.Errorf("purge expired sessions: %w", err)
	}

	return nil
}
