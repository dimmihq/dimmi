/*
|--------------------------------------------------------------------------
| Job Queue
|--------------------------------------------------------------------------
|
| Work that can happen later, or must survive a crash, becomes a job row
| in PostgreSQL. A job pushed inside a transaction exists only if the
| transaction commits: a worker never picks up a job for data that was
| rolled back, and no job is lost for data that was saved.
|
| Every node can push. Only nodes in the worker role start the workers.
| Workers hear about new jobs through LISTEN, which needs a server session
| of its own, so the queue keeps a small pool on a direct connection to
| the primary instead of sharing the application's.
|
*/

package queue

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/log"
)

/*
ErrUnknownKind is returned when a job is pushed whose kind has no
registered worker: nothing would ever run it.
*/
var ErrUnknownKind = errors.New("queue: no worker registered for job kind")

/*
Config configures the queue.

Queues maps each queue name to how many of its jobs one node runs at
once; it defaults to ten workers on the queue named "default". MaxConns
sizes the queue's own pool. JobTimeout bounds one run of a job.
PollInterval is how often idle workers look for jobs no notification
announced, such as scheduled jobs and retries coming due. NoLeader keeps
this node out of leader election, so it never pushes periodic jobs: set
on worker nodes when dedicated scheduler nodes run the schedule.
*/
type Config struct {
	Queues       map[string]int `koanf:"queues"`
	MaxConns     int32          `default:"10" koanf:"max_conns"`
	JobTimeout   time.Duration  `default:"1m" koanf:"job_timeout"`
	PollInterval time.Duration  `default:"1s" koanf:"poll_interval"`
	NoLeader     bool           `koanf:"-"`
}

/*
Queue pushes jobs and, on worker nodes, runs them.

A node that never calls Start only pushes, which is all a web node
needs. Workers are registered with Register before Start.
*/
type Queue struct {
	pool     *pgxpool.Pool
	client   *river.Client[pgx.Tx]
	workers  *river.Workers
	queues   []string
	txMode   pgx.QueryExecMode
	noLeader bool
	running  atomic.Bool
}

/*
New builds a queue on its own pool, connected through the database's
session URL. It connects lazily, so a database that is down does not
stop the boot.
*/
func New(ctx context.Context, db database.Config, cfg Config) (*Queue, error) {
	url, err := db.SessionURL()
	if err != nil {
		return nil, fmt.Errorf("queue connection: %w", err)
	}

	cfg = cfg.withDefaults()

	poolConfig, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse queue url: %w", err)
	}

	poolConfig.MaxConns = cfg.MaxConns

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create queue pool: %w", err)
	}

	q := &Queue{
		pool:     pool,
		workers:  river.NewWorkers(),
		queues:   slices.Collect(maps.Keys(cfg.Queues)),
		txMode:   execMode(db.PoolMode),
		noLeader: cfg.NoLeader,
	}

	q.client, err = river.NewClient(riverpgxv5.New(pool), q.riverConfig(ctx, cfg))
	if err != nil {
		pool.Close()

		return nil, fmt.Errorf("create queue client: %w", err)
	}

	return q, nil
}

/*
Push adds a job on its own, outside any transaction. A job that is
unique and already exists is skipped without an error.
*/
func (q *Queue) Push(ctx context.Context, args Args, opts ...Opt) error {
	_, err := q.client.Insert(ctx, args, insertOpts(args, opts))

	return pushError(args, err)
}

/*
PushTx adds a job inside the transaction. The job exists only if the
transaction commits, and no worker sees it before then.
*/
func (q *Queue) PushTx(ctx context.Context, tx *database.Tx, args Args, opts ...Opt) error {
	_, err := q.client.InsertTx(ctx, riverTx{tx: tx, mode: q.txMode}, args, insertOpts(args, opts))

	return pushError(args, err)
}

/*
Start runs the workers until Stop. Jobs run with contexts derived from
the given one, so ending it cancels every running job.
*/
func (q *Queue) Start(ctx context.Context) error {
	if err := q.client.Start(ctx); err != nil {
		return fmt.Errorf("start queue: %w", err)
	}

	q.running.Store(true)

	return nil
}

/*
Stop stops fetching jobs and waits for running ones to finish. When the
context ends first, the running jobs are canceled and Stop returns the
context's error without waiting further.
*/
func (q *Queue) Stop(ctx context.Context) error {
	err := q.client.Stop(ctx)
	if err == nil {
		q.running.Store(false)

		return nil
	}

	if ctx.Err() == nil {
		return fmt.Errorf("stop queue: %w", err)
	}

	_ = q.client.StopAndCancel(ctx)
	q.running.Store(false)

	return fmt.Errorf("stop queue: %w", ctx.Err())
}

/*
Close closes the queue's pool. Stop the workers first.
*/
func (q *Queue) Close() {
	q.pool.Close()
}

/*
riverConfig translates the config for River, which logs through the
logger the context carries.
*/
func (q *Queue) riverConfig(ctx context.Context, cfg Config) *river.Config {
	queues := make(map[string]river.QueueConfig, len(cfg.Queues))
	for name, workers := range cfg.Queues {
		queues[name] = river.QueueConfig{MaxWorkers: workers}
	}

	return &river.Config{
		Queues:            queues,
		Workers:           q.workers,
		Logger:            log.From(ctx),
		JobTimeout:        cfg.JobTimeout,
		FetchPollInterval: cfg.PollInterval,
		FetchCooldown:     min(cfg.PollInterval, river.FetchCooldownDefault),

		LeaderElectionDisabled: cfg.NoLeader,
	}
}

func (c Config) withDefaults() Config {
	if len(c.Queues) == 0 {
		c.Queues = map[string]int{river.QueueDefault: 10}
	}

	c.MaxConns = cmp.Or(c.MaxConns, 10)
	c.JobTimeout = cmp.Or(c.JobTimeout, time.Minute)
	c.PollInterval = cmp.Or(c.PollInterval, time.Second)

	return c
}

/*
execMode matches how the application's pool runs statements, which
decides how River must encode JSON arguments inside its transactions.
*/
func execMode(mode database.PoolMode) pgx.QueryExecMode {
	if mode == database.PoolModeTransaction {
		return pgx.QueryExecModeExec
	}

	return pgx.QueryExecModeCacheStatement
}

func pushError(args Args, err error) error {
	if err == nil {
		return nil
	}

	if _, ok := errors.AsType[*river.UnknownJobKindError](err); ok {
		return fmt.Errorf("push %s: %w", args.Kind(), ErrUnknownKind)
	}

	return fmt.Errorf("push %s: %w", args.Kind(), err)
}
