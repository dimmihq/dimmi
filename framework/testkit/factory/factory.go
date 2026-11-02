/*
|--------------------------------------------------------------------------
| Factories
|--------------------------------------------------------------------------
|
| A test says only what matters to it about the records it needs; the
| factory fills in the rest with valid values. One definition per type
| builds a record from a sequence number, so every record it makes is
| distinct and parallel tests never collide on unique columns. States
| change what a test cares about on top of the definition.
|
| Records are stored the way the application stores them: through a
| persist function the test supplies, which calls the real queries. The
| factory never writes SQL of its own.
|
*/

package factory

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
)

/*
ErrNoPersist is returned by Create when the factory was given no way to
store its records.
*/
var ErrNoPersist = errors.New("factory: no persist function")

/*
State changes a record built by the definition.
*/
type State[T any] func(record T) T

/*
Persist stores a record and returns it as stored, with the values the
database filled in, such as its id.
*/
type Persist[T any] func(ctx context.Context, record T) (T, error)

/*
Factory builds records of type T. Every option returns a new factory, so
a base factory can be shared and specialized per test. Copies share the
sequence, so records stay distinct whichever copy made them.
*/
type Factory[T any] struct {
	define   func(seq int) T
	seq      *atomic.Int64
	count    int
	states   []State[T]
	sequence []State[T]
	persist  Persist[T]
}

/*
Define returns a factory building records with the function, which
receives a sequence number starting at 1 that is never repeated.
*/
func Define[T any](define func(seq int) T) *Factory[T] {
	return &Factory[T]{define: define, seq: &atomic.Int64{}, count: 1}
}

/*
Count returns a factory making n records at a time.
*/
func (f *Factory[T]) Count(n int) *Factory[T] {
	clone := f.clone()
	clone.count = n

	return clone
}

/*
State returns a factory applying the state to every record, after the
states given before it.
*/
func (f *Factory[T]) State(state State[T]) *Factory[T] {
	clone := f.clone()
	clone.states = append(clone.states, state)

	return clone
}

/*
Sequence returns a factory applying the states in turn: the first to
the first record, the second to the second, starting over when they run
out. They apply after every other state.
*/
func (f *Factory[T]) Sequence(states ...State[T]) *Factory[T] {
	clone := f.clone()
	clone.sequence = states

	return clone
}

/*
Persist returns a factory storing its records with the function.
*/
func (f *Factory[T]) Persist(persist Persist[T]) *Factory[T] {
	clone := f.clone()
	clone.persist = persist

	return clone
}

/*
Make builds the records without storing them.
*/
func (f *Factory[T]) Make() []T {
	records := make([]T, 0, f.count)
	for i := range f.count {
		records = append(records, f.build(i))
	}

	return records
}

/*
MakeOne builds one record without storing it.
*/
func (f *Factory[T]) MakeOne() T {
	return f.build(0)
}

/*
Create builds the records and stores each one, returning them as stored.
It stops at the first record that cannot be stored.
*/
func (f *Factory[T]) Create(ctx context.Context) ([]T, error) {
	if f.persist == nil {
		return nil, ErrNoPersist
	}

	records := f.Make()
	for i, record := range records {
		stored, err := f.persist(ctx, record)
		if err != nil {
			return nil, fmt.Errorf("create record %d of %d: %w", i+1, len(records), err)
		}

		records[i] = stored
	}

	return records, nil
}

/*
CreateOne builds one record and stores it.
*/
func (f *Factory[T]) CreateOne(ctx context.Context) (T, error) {
	records, err := f.Count(1).Create(ctx)
	if err != nil {
		var zero T

		return zero, err
	}

	return records[0], nil
}

/*
build makes the record at the index of one batch.
*/
func (f *Factory[T]) build(index int) T {
	record := f.define(int(f.seq.Add(1)))
	for _, state := range f.states {
		record = state(record)
	}

	if len(f.sequence) > 0 {
		record = f.sequence[index%len(f.sequence)](record)
	}

	return record
}

func (f *Factory[T]) clone() *Factory[T] {
	clone := *f
	clone.states = append([]State[T](nil), f.states...)

	return &clone
}
