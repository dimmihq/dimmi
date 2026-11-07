/*
|--------------------------------------------------------------------------
| Visitor Sign-In Upkeep
|--------------------------------------------------------------------------
|
| Two rounds keep visitor sign-in healthy. Every hour the auth provider
| key sets whose cached copy is six hours old are fetched again, so a
| provider's key rotation is picked up before its old key goes away;
| a failure is noted on the secret for the dashboard and the copy is
| kept. Every night the daily sign-in counts past ninety days go.
|
*/

package jobs

import (
	"context"

	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app/actions/identity"
)

/*
maxKeySetsPerRound bounds the key sets one round fetches; the next round
picks up the rest.
*/
const maxKeySetsPerRound = 500

/*
RefreshIdentityKeySetsArgs carries nothing: every run refreshes every
key set due.
*/
type RefreshIdentityKeySetsArgs struct{}

/*
Kind names the job.
*/
func (RefreshIdentityKeySetsArgs) Kind() string {
	return "refresh_identity_key_sets"
}

/*
Options try the job a few times; the next hour's run catches up anyway.
*/
func (RefreshIdentityKeySetsArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: expiryAttempts}
}

/*
RefreshIdentityKeySets fetches the key sets due again.
*/
type RefreshIdentityKeySets struct {
	Sets *identity.KeySets
}

/*
Handle refreshes the key sets due.
*/
func (w *RefreshIdentityKeySets) Handle(ctx context.Context, _ *queue.Job[RefreshIdentityKeySetsArgs]) error {
	_, err := w.Sets.RefreshDue(ctx, maxKeySetsPerRound)

	return err
}

/*
PruneIdentityStatsArgs carries nothing: every run prunes every
workspace.
*/
type PruneIdentityStatsArgs struct{}

/*
Kind names the job.
*/
func (PruneIdentityStatsArgs) Kind() string {
	return "prune_identity_stats"
}

/*
Options try the job a few times; the next night's run catches up anyway.
*/
func (PruneIdentityStatsArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: expiryAttempts}
}

/*
PruneIdentityStats drops sign-in counts past their keep.
*/
type PruneIdentityStats struct {
	Stats *identity.Stats
}

/*
Handle prunes the counts.
*/
func (w *PruneIdentityStats) Handle(ctx context.Context, _ *queue.Job[PruneIdentityStatsArgs]) error {
	_, err := w.Stats.Prune(ctx)

	return err
}
