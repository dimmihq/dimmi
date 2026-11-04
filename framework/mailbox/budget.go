/*
|--------------------------------------------------------------------------
| Memory Budget
|--------------------------------------------------------------------------
|
| Mail arrives in bursts and a single message can reach tens of
| megabytes, while the smallest install runs in 256 MB. Everything that
| holds a whole message in memory shares one budget. A message counts for
| twice its size, the copy being read and what parsing makes of it, and
| never for more than the whole budget, so even the largest message gets
| through alone.
|
*/

package mailbox

import (
	"context"
	"errors"
	"fmt"
	"time"

	"golang.org/x/sync/semaphore"
)

/*
ErrBudgetBusy means the memory budget stayed full for as long as the
caller was willing to wait.
*/
var ErrBudgetBusy = errors.New("mailbox: memory budget busy")

/*
Budget is a bound on the bytes of mail held in memory at once.
*/
type Budget struct {
	total int64
	sem   *semaphore.Weighted
}

/*
NewBudget returns a budget of the given bytes.
*/
func NewBudget(bytes int64) *Budget {
	return &Budget{total: bytes, sem: semaphore.NewWeighted(bytes)}
}

/*
Acquire waits up to the given time for room for a message of the size,
and returns the function giving the room back. A size below zero is
unknown and counts as the whole budget.
*/
func (b *Budget) Acquire(ctx context.Context, size int64, wait time.Duration) (func(), error) {
	weight := b.weight(size)

	ctx, cancel := context.WithTimeout(ctx, wait)
	defer cancel()

	if err := b.sem.Acquire(ctx, weight); err != nil {
		return nil, fmt.Errorf("%w: %d bytes", ErrBudgetBusy, weight)
	}

	return func() { b.sem.Release(weight) }, nil
}

func (b *Budget) weight(size int64) int64 {
	if size < 0 || size*2 > b.total {
		return b.total
	}

	return max(size*2, 1)
}
