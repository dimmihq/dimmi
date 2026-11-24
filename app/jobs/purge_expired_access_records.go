/*
|--------------------------------------------------------------------------
| Purge Expired Access Records
|--------------------------------------------------------------------------
|
| Some records only ever let someone in, and mean nothing once they no
| longer can: anonymous messenger sessions past their expiry, invitations
| that expired or were revoked, access tokens and connected apps' tokens
| that expired or were revoked, and the stored answers of idempotency
| keys past their time. Every night they are deleted, so they neither
| pile up nor linger as personal data nobody needs.
|
| Each record already stopped working by its own timestamps; deleting it
| changes nothing anyone can do. Invitations and tokens are kept for 30
| days after they ended, so a teammate still sees what expired or who
| revoked what, and can send an invitation again.
|
*/

package jobs

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app/models"
)

/*
EndedAccessRetention is how long an invitation or a token is kept after
it expired or was revoked.
*/
const EndedAccessRetention = 30 * day

/*
IdempotencyPruner deletes the idempotency keys past their expiry.
*/
type IdempotencyPruner interface {
	/*
		Prune deletes expired keys and returns how many went.
	*/
	Prune(ctx context.Context) (int64, error)
}

/*
PurgeExpiredAccessRecordsArgs starts one purge.
*/
type PurgeExpiredAccessRecordsArgs struct{}

/*
Kind names the job.
*/
func (PurgeExpiredAccessRecordsArgs) Kind() string {
	return "purge_expired_access_records"
}

/*
Options try the job a few times and run it once a day however many
times it is pushed: the next run catches up on anything left.
*/
func (PurgeExpiredAccessRecordsArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: pruneAttempts, Unique: queue.UniqueOpts{ByPeriod: day}}
}

/*
PurgeExpiredAccessRecords deletes the access records that ended.
Idempotency, when set, prunes the idempotency keys.
*/
type PurgeExpiredAccessRecords struct {
	DB          *database.DB
	Clock       clock.Clock
	Idempotency IdempotencyPruner
}

/*
Handle deletes every kind of ended record, a batch at a time.
*/
func (w *PurgeExpiredAccessRecords) Handle(ctx context.Context, _ *queue.Job[PurgeExpiredAccessRecordsArgs]) error {
	q := models.New(w.DB)
	now := w.Clock.Now()
	ended := now.Add(-EndedAccessRetention)

	err := prune(ctx, func(ctx context.Context) (int64, error) {
		return q.PurgeExpiredWidgetSessions(ctx, models.PurgeExpiredWidgetSessionsParams{
			ExpiredBefore: now, MaxRows: pruneBatch,
		})
	})
	if err != nil {
		return fmt.Errorf("purge expired messenger sessions: %w", err)
	}

	err = prune(ctx, func(ctx context.Context) (int64, error) {
		return q.PurgeExpiredInvitations(ctx, models.PurgeExpiredInvitationsParams{
			EndedBefore: ended, MaxRows: pruneBatch,
		})
	})
	if err != nil {
		return fmt.Errorf("purge expired invitations: %w", err)
	}

	err = prune(ctx, func(ctx context.Context) (int64, error) {
		return q.PurgeDeadAccessTokens(ctx, models.PurgeDeadAccessTokensParams{EndedBefore: ended, MaxRows: pruneBatch})
	})
	if err != nil {
		return fmt.Errorf("purge dead access tokens: %w", err)
	}

	if _, err := q.DeleteDeadOAuthTokens(ctx, ended); err != nil {
		return fmt.Errorf("purge dead oauth tokens: %w", err)
	}

	if w.Idempotency == nil {
		return nil
	}

	if _, err := w.Idempotency.Prune(ctx); err != nil {
		return fmt.Errorf("purge idempotency keys: %w", err)
	}

	return nil
}
