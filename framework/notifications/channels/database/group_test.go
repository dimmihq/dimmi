package database_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/database/migrate"
	"github.com/dimmihq/dimmi/framework/notifications"
	dbchannel "github.com/dimmihq/dimmi/framework/notifications/channels/database"
	"github.com/dimmihq/dimmi/framework/testkit"
)

/*
replied is a grouped notification of a tenant: every reply in one
thread folds into one unread row.
*/
type replied struct {
	Thread string
	Body   string
	Tenant string
}

func (replied) Type() string { return "replied" }

func (replied) Via(notifications.Notifiable) []string { return []string{"database"} }

func (n replied) ToDatabase(context.Context, notifications.Delivery) (any, error) {
	return map[string]string{"body": n.Body}, nil
}

func (n replied) GroupKey(notifications.Delivery) string { return n.Thread }

func (n replied) TenantID() string { return n.Tenant }

/*
grouped is what a test checks about a grouped row.
*/
type grouped struct {
	Count  int
	Data   string
	Tenant *string
	Seen   bool
}

func (f fixture) grouped(t *testing.T) []grouped {
	t.Helper()
	rs, err := f.db.Query(t.Context(), `SELECT count, data->>'body', tenant_id, seen_at IS NOT NULL FROM alerts
		WHERE notifiable_id = 'ada' ORDER BY created_at, id`)
	require.NoError(t, err)
	defer rs.Close()
	var out []grouped
	for rs.Next() {
		var g grouped
		require.NoError(t, rs.Scan(&g.Count, &g.Data, &g.Tenant, &g.Seen))
		out = append(out, g)
	}
	return out
}

func TestGroupedNotifications(t *testing.T) {
	t.Parallel()

	t.Run("folds occurrences into one unread row per group", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)
		ada := member{ID: "ada"}
		ctx := t.Context()

		send := func(id, thread, body string) {
			require.NoError(t, f.channel.Send(ctx, notifications.Delivery{ID: id, Notifiable: ada},
				replied{Thread: thread, Body: body, Tenant: "ws_1"}))
		}
		send("01890000-0000-7000-8000-000000000001", "t1", "first")
		send("01890000-0000-7000-8000-000000000002", "t1", "second")
		send("01890000-0000-7000-8000-000000000002", "t1", "second again")
		send("01890000-0000-7000-8000-000000000003", "t2", "other thread")

		rows := f.grouped(t)
		require.Len(t, rows, 2)
		assert.Equal(t, 2, rows[0].Count)
		assert.Equal(t, "second", rows[0].Data)
		assert.Equal(t, "ws_1", *rows[0].Tenant)
		assert.Equal(t, 1, rows[1].Count)

		require.NoError(t, f.channel.MarkAllRead(ctx, ada))
		send("01890000-0000-7000-8000-000000000004", "t1", "after reading")
		assert.Len(t, f.grouped(t), 3)
	})

	t.Run("marks only what changed by now seen, and a new occurrence unseen again", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)
		ada := member{ID: "ada"}
		ctx := t.Context()
		require.NoError(t, f.channel.Send(ctx, notifications.Delivery{
			ID:         "01890000-0000-7000-8000-000000000001",
			Notifiable: ada,
		}, replied{Thread: "t1", Body: "one"}))
		require.NoError(t, f.channel.MarkSeen(ctx, ada))
		assert.True(t, f.grouped(t)[0].Seen)
		assert.Nil(t, f.grouped(t)[0].Tenant)

		f.clock.Advance(time.Minute)
		require.NoError(t, f.channel.Send(ctx, notifications.Delivery{
			ID:         "01890000-0000-7000-8000-000000000002",
			Notifiable: ada,
		}, replied{Thread: "t1", Body: "two"}))
		assert.False(t, f.grouped(t)[0].Seen)
		require.NoError(t, f.channel.MarkSeen(ctx, member{ID: "bob"}))
		assert.False(t, f.grouped(t)[0].Seen)
	})

	t.Run("reports a failed seen update", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)
		_, err := f.db.Exec(t.Context(), `DROP TABLE alerts`)
		require.NoError(t, err)
		require.Error(t, f.channel.MarkSeen(t.Context(), member{ID: "ada"}))
	})

	t.Run("adds the newer columns to a table made by an older release", func(t *testing.T) {
		t.Parallel()
		url := testkit.NewDatabase(t).URL
		f := fixture{url: url}
		pool := poolFor(t, f)
		_, err := pool.Exec(t.Context(), `CREATE TABLE alerts (id uuid PRIMARY KEY, notifiable_type text NOT NULL,
			notifiable_id text NOT NULL, type text NOT NULL, data jsonb NOT NULL DEFAULT '{}', read_at timestamptz,
			created_at timestamptz NOT NULL DEFAULT now())`)
		require.NoError(t, err)
		_, err = migrate.Runner{URL: url, Steps: []migrate.Step{dbchannel.Migration("alerts")}}.Up(t.Context())
		require.NoError(t, err)
		var columns int
		require.NoError(t, pool.QueryRow(t.Context(), `SELECT count(*) FROM information_schema.columns
			WHERE table_name = 'alerts' AND column_name IN ('tenant_id','group_key','count','seen_at','updated_at')`).
			Scan(&columns))
		assert.Equal(t, 5, columns)
	})

	t.Run("keeps the sqlc schema file in step with the migration", func(t *testing.T) {
		t.Parallel()
		raw, err := os.ReadFile("schema.sql")
		require.NoError(t, err)
		for _, column := range []string{
			"tenant_id text", "group_key text", "count integer NOT NULL DEFAULT 1",
			"last_delivery_id text", "seen_at timestamptz", "updated_at timestamptz NOT NULL DEFAULT now()",
		} {
			assert.True(t, strings.Contains(string(raw), column), column)
		}
	})
}
