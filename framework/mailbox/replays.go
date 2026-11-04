/*
|--------------------------------------------------------------------------
| Replay Window
|--------------------------------------------------------------------------
|
| A signed delivery captured on the way can be posted again, so sources
| whose provider signs a token remember the token for as long as its
| signature stays valid. A token is remembered only after the raw message
| is kept, never before: an attempt that failed halfway must stay
| retryable. When the store is down the delivery is refused for now, and
| the provider retries.
|
*/

package mailbox

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/cache"
)

/*
replayWindow outlasts every signature window a source accepts.
*/
const replayWindow = 10 * time.Minute

/*
Replays remembers the signed deliveries sources have accepted.
*/
type Replays struct {
	store *cache.Store
}

/*
NewReplays returns a replay window on the store.
*/
func NewReplays(store *cache.Store) *Replays {
	return &Replays{store: store}
}

/*
Check rejects a token already accepted for the source.
*/
func (r *Replays) Check(ctx context.Context, source, token string) error {
	_, err := r.store.Get(ctx, replayKey(source, token))
	if errors.Is(err, cache.ErrMiss) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("check replay: %w", err)
	}

	return fmt.Errorf("%w: delivery seen before", ErrRejected)
}

/*
Accepted returns the function an envelope carries to remember the token
once the raw message is kept.
*/
func (r *Replays) Accepted(source, token string) func(context.Context) error {
	return func(ctx context.Context) error {
		_, err := r.store.Add(ctx, replayKey(source, token), []byte("1"), replayWindow)
		if err != nil {
			return fmt.Errorf("remember delivery: %w", err)
		}

		return nil
	}
}

func replayKey(source, token string) string {
	return "mailbox:replay:" + source + ":" + token
}
