/*
|--------------------------------------------------------------------------
| Hedged Calls
|--------------------------------------------------------------------------
|
| A search waits on one small call to a hosted embedder, and a hosted
| embedder answers most calls quickly and a few very slowly. A search
| that falls back to keywords because of one slow call answers worse,
| so the slow tail is cut instead: when the first call has not answered
| after a short wait, a second identical one is sent and whichever
| answers first is used. A call that fails outright is tried again at
| once. The extra call is made only for the slow few, so it costs
| almost nothing.
|
*/

package knowledge

import (
	"context"
	"time"
)

/*
attempt is how one call came out.
*/
type attempt[T any] struct {
	value T
	err   error
}

/*
hedged runs call and returns its first success. When the first call
has not answered after wait, a second is started and the loser is
canceled; when the first fails outright, it is tried once more. It
fails only when both did, with the first error. A wait of zero or less
makes one call and no second.
*/
func hedged[T any](ctx context.Context, wait time.Duration, call func(context.Context) (T, error)) (T, error) {
	if wait <= 0 {
		return call(ctx)
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	results := make(chan attempt[T], 2)
	run := func() {
		value, err := call(ctx)
		results <- attempt[T]{value, err}
	}
	go run()

	slow, stop := context.WithTimeout(ctx, wait)
	defer stop()
	select {
	case first := <-results:
		if first.err == nil {
			return first.value, nil
		}
		if value, err := call(ctx); err == nil {
			return value, nil
		}
		return first.value, first.err
	case <-slow.Done():
	}

	go run()
	return firstOf(results, 2)
}

/*
firstOf waits for up to n attempts and returns the first that
succeeded, or the first error when none did.
*/
func firstOf[T any](results <-chan attempt[T], n int) (T, error) {
	var failed *attempt[T]
	for range n {
		got := <-results
		if got.err == nil {
			return got.value, nil
		}
		if failed == nil {
			failed = &got
		}
	}
	return failed.value, failed.err
}
