package database_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/database/migrate"
	"github.com/dimmihq/dimmi/framework/notifications"
	dbchannel "github.com/dimmihq/dimmi/framework/notifications/channels/database"
	"github.com/dimmihq/dimmi/framework/testkit"
)

type member struct {
	ID string
}

func (member) NotifiableType() string { return "member" }

func (m member) NotifiableID() string { return m.ID }

func (member) RouteFor(string) any { return nil }

type invoicePaid struct {
	Amount int
	Fail   bool
	Bad    bool
}

func (invoicePaid) Type() string { return "invoice_paid" }

func (invoicePaid) Via(notifications.Notifiable) []string { return []string{"database"} }

func (n invoicePaid) ToDatabase(context.Context, notifications.Delivery) (any, error) {
	if n.Fail {
		return nil, errors.New("invoice missing")
	}

	if n.Bad {
		return make(chan int), nil
	}

	return map[string]int{"amount": n.Amount}, nil
}

/*
silent has no database rendering.
*/
type silent struct{}

func (silent) Type() string { return "silent" }

func (silent) Via(notifications.Notifiable) []string { return []string{"database"} }

/*
row is what a test checks about a stored notification.
*/
type row struct {
	ID             string
	NotifiableType string
	NotifiableID   string
	Type           string
	Data           string
	Read           bool
	CreatedAt      time.Time
}

/*
fixture is a migrated database with a channel storing into "alerts".
*/
type fixture struct {
	url     string
	db      *database.DB
	clock   *clock.Fake
	channel *dbchannel.Channel
	manager *notifications.Manager
}

func newFixture(t *testing.T) fixture {
	t.Helper()

	url := testkit.NewDatabase(t).URL
	_, err := migrate.Runner{URL: url, Steps: []migrate.Step{dbchannel.Migration("alerts")}}.Up(t.Context())
	require.NoError(t, err)

	db, err := database.Open(t.Context(), database.Config{URL: url, PoolMode: database.PoolModeSession, MaxConns: 2})
	require.NoError(t, err)
	t.Cleanup(db.Close)

	clk := clock.NewFake(time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC))

	channel, err := dbchannel.New(db, clk, dbchannel.Config{Table: "alerts"})
	require.NoError(t, err)

	m, err := notifications.New(notifications.Config{
		Channels: map[string]config.Section{"database": {"driver": "database", "table": "alerts"}},
	}, nil)
	require.NoError(t, err)
	m.Extend("database", dbchannel.Factory(db, clk))

	return fixture{url: url, db: db, clock: clk, channel: channel, manager: m}
}

/*
poolFor opens a one-connection pool on the fixture's database, as the
migration runner hands its steps.
*/
func poolFor(t *testing.T, f fixture) *pgxpool.Pool {
	t.Helper()

	cfg, err := pgxpool.ParseConfig(f.url)
	require.NoError(t, err)
	cfg.MaxConns = 1

	pool, err := pgxpool.NewWithConfig(t.Context(), cfg)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	return pool
}

/*
rows reads every stored notification, oldest first.
*/
func (f fixture) rows(t *testing.T) []row {
	t.Helper()

	rs, err := f.db.Query(t.Context(), `SELECT id::text, notifiable_type, notifiable_id, type, data::text,
		read_at IS NOT NULL, created_at FROM alerts ORDER BY created_at, id`)
	require.NoError(t, err)

	all, err := pgx.CollectRows(rs, func(r pgx.CollectableRow) (row, error) {
		var x row
		err := r.Scan(&x.ID, &x.NotifiableType, &x.NotifiableID, &x.Type, &x.Data, &x.Read, &x.CreatedAt)
		x.CreatedAt = x.CreatedAt.UTC()

		return x, err
	})
	require.NoError(t, err)

	return all
}

func TestSend(t *testing.T) {
	t.Parallel()

	t.Run("stores the rendered data for the recipient", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)

		require.NoError(t, f.manager.Send(t.Context(), invoicePaid{Amount: 42}, member{ID: "m1"}))

		rows := f.rows(t)
		require.Len(t, rows, 1)
		assert.Equal(t, row{
			ID: rows[0].ID, NotifiableType: "member", NotifiableID: "m1", Type: "invoice_paid",
			Data: `{"amount": 42}`, CreatedAt: f.clock.Now(),
		}, rows[0])
	})

	t.Run("stores one row when a delivery runs twice", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)
		d := notifications.Delivery{
			ID: "0196a1b2-0000-7000-8000-000000000001", Channel: "database",
			Notifiable: member{ID: "m1"},
		}

		require.NoError(t, f.channel.Send(t.Context(), d, invoicePaid{Amount: 1}))
		require.NoError(t, f.channel.Send(t.Context(), d, invoicePaid{Amount: 1}))

		assert.Len(t, f.rows(t), 1)
	})

	t.Run("refuses what it cannot store", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)
		to := member{ID: "m1"}

		require.ErrorIs(t, f.manager.Send(t.Context(), silent{}, to), notifications.ErrUnsupported)
		require.ErrorContains(t, f.manager.Send(t.Context(), invoicePaid{Fail: true}, to),
			"render invoice_paid for database: invoice missing")
		require.ErrorIs(t, f.manager.Send(t.Context(), invoicePaid{Bad: true}, to), notifications.ErrPermanent)
		assert.Empty(t, f.rows(t))
	})

	t.Run("reports a failed insert", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)
		channel, err := dbchannel.New(f.db, f.clock, dbchannel.Config{Table: "missing"})
		require.NoError(t, err)

		err = channel.Send(t.Context(), notifications.Delivery{ID: "x", Notifiable: member{ID: "m1"}}, invoicePaid{})

		require.ErrorContains(t, err, "store invoice_paid")
	})
}

func TestMarkRead(t *testing.T) {
	t.Parallel()

	t.Run("marks only the recipient's own notifications", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)
		ada, bo := member{ID: "ada"}, member{ID: "bo"}
		require.NoError(t, f.manager.Send(t.Context(), invoicePaid{Amount: 1}, ada, bo))
		f.clock.Travel(time.Second)
		require.NoError(t, f.manager.Send(t.Context(), invoicePaid{Amount: 2}, ada))
		rows := f.rows(t)

		require.NoError(t, f.channel.MarkRead(t.Context(), bo, rows[0].ID, rows[1].ID, rows[2].ID))

		assert.Equal(t, []bool{false, true, false}, read(f.rows(t)))

		require.NoError(t, f.channel.MarkAllRead(t.Context(), ada))
		assert.Equal(t, []bool{true, true, true}, read(f.rows(t)))

		require.NoError(t, f.channel.MarkUnread(t.Context(), ada, rows[2].ID, rows[1].ID))
		assert.Equal(t, []bool{true, true, false}, read(f.rows(t)))
	})

	t.Run("reports a failed update", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)
		ada := member{ID: "ada"}

		require.ErrorContains(t, f.channel.MarkRead(t.Context(), ada, "not-a-uuid"), "mark notifications read")
		require.ErrorContains(t, f.channel.MarkUnread(t.Context(), ada, "not-a-uuid"), "mark notifications unread")

		missing, err := dbchannel.New(f.db, f.clock, dbchannel.Config{Table: "missing"})
		require.NoError(t, err)
		require.ErrorContains(t, missing.MarkAllRead(t.Context(), ada), "mark all notifications read")
	})
}

func TestNew(t *testing.T) {
	t.Parallel()

	t.Run("refuses a table name that is not a plain identifier", func(t *testing.T) {
		t.Parallel()

		tables := []string{"", "Alerts", "alerts; drop table users", "a" + strings.Repeat("b", 45)}
		for _, table := range tables {
			_, err := dbchannel.New(nil, clock.System(), dbchannel.Config{Table: table})

			require.ErrorIs(t, err, dbchannel.ErrInvalidTable, table)
		}
	})

	t.Run("reads the table from the channel's configuration", func(t *testing.T) {
		t.Parallel()
		factory := dbchannel.Factory(nil, clock.System())

		_, err := factory("database", config.Section{})
		require.NoError(t, err)

		_, err = factory("database", config.Section{"table": "Bad"})
		require.ErrorIs(t, err, dbchannel.ErrInvalidTable)

		_, err = factory("database", config.Section{"table": map[string]any{"nested": true}})
		require.ErrorContains(t, err, "notifications database")
	})
}

func TestMigration(t *testing.T) {
	t.Parallel()

	t.Run("creates the table once and leaves it on later runs", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)
		require.NoError(t, f.manager.Send(t.Context(), invoicePaid{Amount: 1}, member{ID: "m1"}))

		step := dbchannel.Migration("alerts")
		require.NoError(t, step.Up(t.Context(), poolFor(t, f)))

		assert.Len(t, f.rows(t), 1)
	})

	t.Run("refuses an invalid table name", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)

		err := dbchannel.Migration("Bad").Up(t.Context(), poolFor(t, f))

		require.ErrorIs(t, err, dbchannel.ErrInvalidTable)
	})

	t.Run("reports a failed create", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)
		pool := poolFor(t, f)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		err := dbchannel.Migration("others").Up(ctx, pool)

		require.ErrorContains(t, err, "create notifications table others")
	})
}

func read(rows []row) []bool {
	out := make([]bool, len(rows))
	for i, r := range rows {
		out[i] = r.Read
	}

	return out
}
