/*
|--------------------------------------------------------------------------
| Read Replicas
|--------------------------------------------------------------------------
|
| Replicas take load off the primary for reads that can be a few seconds
| old. A replica is only used while it is reachable and close enough to
| the primary; each one is checked on a fixed beat, and a replica that
| falls behind or stops answering is skipped until it recovers.
|
| A replica is caught up when everything it received has been replayed.
| Otherwise its lag is the time since the last transaction it replayed.
| A server that is not in recovery has no lag at all.
|
*/

package database

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dimmihq/dimmi/framework/clock"
)

/*
replicaCheckInterval is how often every replica's lag is measured, and
how long one measurement may take.
*/
const replicaCheckInterval = 5 * time.Second

/*
lagSQL returns the replica's lag in seconds, or NULL when it cannot be
known because nothing has been replayed yet.
*/
const lagSQL = `SELECT CASE
	WHEN NOT pg_is_in_recovery() THEN 0
	WHEN pg_last_wal_receive_lsn() = pg_last_wal_replay_lsn() THEN 0
	ELSE extract(epoch FROM now() - pg_last_xact_replay_timestamp())
END::float8`

var errLagUnknown = errors.New("replica lag unknown")

type replica struct {
	pool    *pgxpool.Pool
	healthy atomic.Bool
}

type replicaSet struct {
	replicas []*replica
	maxLag   time.Duration
	next     atomic.Uint32
	stop     context.CancelFunc
	done     chan struct{}
}

/*
openReplicas builds a pool for every read URL and starts checking them.
It returns nil when no replica is configured.
*/
func openReplicas(ctx context.Context, cfg Config, clk clock.Clock) (*replicaSet, error) {
	if len(cfg.ReadURLs) == 0 {
		return nil, nil
	}

	set := &replicaSet{maxLag: cfg.ReadMaxLag, done: make(chan struct{})}
	for i, url := range cfg.ReadURLs {
		pool, err := newPool(ctx, url, cfg)
		if err != nil {
			set.closePools()

			return nil, fmt.Errorf("read url %d: %w", i, err)
		}

		set.replicas = append(set.replicas, &replica{pool: pool})
	}

	watchCtx, stop := context.WithCancel(context.WithoutCancel(ctx))
	set.stop = stop

	go set.watch(watchCtx, clk)

	return set, nil
}

/*
pick returns the next healthy replica in turn, or nil when there is none.
*/
func (s *replicaSet) pick() *replica {
	if s == nil {
		return nil
	}

	start := int(s.next.Add(1))
	for i := range s.replicas {
		r := s.replicas[(start+i)%len(s.replicas)]
		if r.healthy.Load() {
			return r
		}
	}

	return nil
}

/*
watch checks every replica at once, then again on every tick until the
set is closed.
*/
func (s *replicaSet) watch(ctx context.Context, clk clock.Clock) {
	defer close(s.done)

	ticker := clk.NewTicker(replicaCheckInterval)
	defer ticker.Stop()

	for {
		s.check(ctx)

		select {
		case <-ctx.Done():
			return
		case <-ticker.Chan():
		}
	}
}

func (s *replicaSet) check(ctx context.Context) {
	for _, r := range s.replicas {
		lag, err := measureLag(ctx, r.pool)
		r.healthy.Store(err == nil && lag <= s.maxLag)
	}
}

func measureLag(ctx context.Context, pool *pgxpool.Pool) (time.Duration, error) {
	ctx, cancel := context.WithTimeout(ctx, replicaCheckInterval)
	defer cancel()

	var seconds *float64
	if err := pool.QueryRow(ctx, lagSQL).Scan(&seconds); err != nil {
		return 0, fmt.Errorf("measure replica lag: %w", err)
	}

	if seconds == nil {
		return 0, errLagUnknown
	}

	return time.Duration(*seconds * float64(time.Second)), nil
}

func (s *replicaSet) close() {
	if s == nil {
		return
	}

	s.stop()
	<-s.done
	s.closePools()
}

func (s *replicaSet) closePools() {
	for _, r := range s.replicas {
		r.pool.Close()
	}
}
