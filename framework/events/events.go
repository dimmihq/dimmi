/*
|--------------------------------------------------------------------------
| Event Dispatcher
|--------------------------------------------------------------------------
|
| An action records what happened as an event, inside the transaction
| that made it happen. Each listener picks the moment it needs:
|
| In the transaction: it runs now, writes with the action, and may fail
| the whole action, so it must stay fast and never call the network.
|
| After commit: it runs once the data is saved, such as a broadcast to
| open screens. It cannot undo anything, so its failure is logged only.
|
| Queued: it becomes a job pushed in the same transaction, so the job
| exists exactly when the data does, and runs later on a worker.
|
| A rolled-back transaction leaves no trace: no after-commit listener
| runs and no job exists.
|
*/

package events

import (
	"context"
	"fmt"
	"sync"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/log"
	"github.com/dimmihq/dimmi/framework/queue"
)

/*
Event is anything that happened. Listeners pick events by Go type, or by
an interface the events implement.
*/
type Event = any

/*
Phase is when a listener runs relative to the transaction.
*/
type Phase int

/*
The listener phases, described on the package: InTx runs inside the
transaction and may fail it, AfterCommit runs once it has committed.
*/
const (
	InTx Phase = iota + 1
	AfterCommit
)

/*
Dispatcher hands events to their listeners. Listeners are registered at
boot, before the first dispatch.
*/
type Dispatcher struct {
	queue *queue.Queue

	mu        sync.RWMutex
	listeners map[Phase][]handler
	queued    []func(e Event) queue.Args
}

/*
handler runs a listener for an event, and does nothing for an event of
another type.
*/
type handler func(ctx context.Context, e Event) error

/*
New returns a dispatcher that pushes the jobs of queued listeners onto
the queue.
*/
func New(q *queue.Queue) *Dispatcher {
	return &Dispatcher{queue: q, listeners: make(map[Phase][]handler)}
}

/*
Listen runs the listener for every event of type E in the phase, InTx
or AfterCommit. E may be an interface, to hear every event that
implements it. The listener receives the dispatching context, so an
in-transaction listener's queries join the transaction.
*/
func Listen[E Event](d *Dispatcher, phase Phase, fn func(ctx context.Context, e E) error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.listeners[phase] = append(d.listeners[phase], func(ctx context.Context, e Event) error {
		typed, ok := e.(E)
		if !ok {
			return nil
		}

		return fn(ctx, typed)
	})
}

/*
ListenQueued pushes the job the function returns for every event of
type E, in the dispatching transaction. Returning nil pushes nothing.
*/
func ListenQueued[E Event](d *Dispatcher, fn func(e E) queue.Args) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.queued = append(d.queued, func(e Event) queue.Args {
		typed, ok := e.(E)
		if !ok {
			return nil
		}

		return fn(typed)
	})
}

/*
Dispatch hands the event to its listeners. In-transaction listeners run
first and in order; the first error stops the dispatch and is returned,
so the caller's transaction rolls back. Queued listeners' jobs are then
pushed in the transaction, and after-commit listeners are held until it
commits.

With a nil transaction, every listener runs now and jobs are pushed on
their own. Ephemeral events skip the in-transaction and queued
listeners.
*/
func (d *Dispatcher) Dispatch(ctx context.Context, tx *database.Tx, e Event) error {
	d.mu.RLock()
	inTx, afterCommit, queued := d.listeners[InTx], d.listeners[AfterCommit], d.queued
	d.mu.RUnlock()

	if !IsEphemeral(e) {
		for _, handle := range inTx {
			if err := handle(ctx, e); err != nil {
				return fmt.Errorf("dispatch %T: %w", e, err)
			}
		}

		if err := d.push(ctx, tx, queued, e); err != nil {
			return err
		}
	}

	if tx == nil {
		runAfterCommit(ctx, afterCommit, e)

		return nil
	}

	tx.AfterCommit(func(ctx context.Context) { runAfterCommit(ctx, afterCommit, e) })

	return nil
}

/*
push pushes the job of every queued listener that has one for the event.
*/
func (d *Dispatcher) push(ctx context.Context, tx *database.Tx, queued []func(e Event) queue.Args, e Event) error {
	for _, job := range queued {
		args := job(e)
		if args == nil {
			continue
		}

		var err error
		if tx == nil {
			err = d.queue.Push(ctx, args)
		} else {
			err = d.queue.PushTx(ctx, tx, args)
		}

		if err != nil {
			return fmt.Errorf("dispatch %T: %w", e, err)
		}
	}

	return nil
}

/*
runAfterCommit runs every after-commit listener. The data is already
saved, so a failure is logged and the remaining listeners still run.
*/
func runAfterCommit(ctx context.Context, handlers []handler, e Event) {
	for _, handle := range handlers {
		if err := handle(ctx, e); err != nil {
			log.From(ctx).ErrorContext(ctx, "after-commit listener failed",
				log.String("event", fmt.Sprintf("%T", e)), log.Err(err))
		}
	}
}
