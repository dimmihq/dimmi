package rest_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/http/rest"
	"github.com/dimmihq/dimmi/framework/testkit"
)

func openDatabase(t *testing.T) *database.DB {
	t.Helper()

	db, err := database.Open(t.Context(), database.Config{
		URL:           testkit.NewDatabase(t).URL,
		MaxConns:      4,
		PoolMode:      database.PoolModeSession,
		TenantSetting: "app.tenant_id",
	})
	require.NoError(t, err)
	t.Cleanup(db.Close)

	_, err = db.Exec(t.Context(), `
		CREATE TABLE tasks (id text PRIMARY KEY, title text NOT NULL, status text NOT NULL);
		INSERT INTO tasks VALUES ('tkt_1', 'Printer', 'open')`)
	require.NoError(t, err)

	return db
}

func changes(t *testing.T, body string) rest.Changes {
	t.Helper()

	var out rest.Changes
	require.NoError(t, json.Unmarshal([]byte(body), &out))

	return out
}

/*
tasks wires a dispatcher on the tasks table: titles are plain,
status has its own handler taking a reason, and closing is refused.
*/
func tasks(db *database.DB, seen map[string]rest.Changes) rest.Dispatcher[string] {
	return rest.Dispatcher[string]{
		Plain: func(ctx context.Context, id string, c rest.Changes) error {
			seen["plain"] = c
			var title string
			if err := json.Unmarshal(c["title"], &title); err != nil {
				return err
			}
			_, err := db.Exec(ctx, "UPDATE tasks SET title = $1 WHERE id = $2", title, id)

			return err
		},
		Fields: map[string]rest.FieldHandler[string]{
			"status": func(ctx context.Context, id string, c rest.Changes) error {
				seen["status"] = c
				if string(c["status"]) == `"closed"` {
					return fault.New(fault.Invalid, "invalid_transition", "Cannot close.")
				}
				_, err := db.Exec(ctx, "UPDATE tasks SET status = 'paused' WHERE id = $1", id)

				return err
			},
		},
		Params: map[string][]string{"status": {"reason"}},
		DB:     db,
	}
}

func row(t *testing.T, db *database.DB) (string, string) {
	t.Helper()

	var title, status string
	require.NoError(t, db.QueryRow(t.Context(), "SELECT title, status FROM tasks").Scan(&title, &status))

	return title, status
}

func TestDispatcher(t *testing.T) {
	t.Parallel()

	t.Run("sends state fields and their parameters to their handler", func(t *testing.T) {
		t.Parallel()
		db := openDatabase(t)
		seen := map[string]rest.Changes{}

		err := tasks(db, seen).Update(t.Context(), "tkt_1",
			changes(t, `{"title":"Scanner","status":"paused","reason":"waiting"}`))
		require.NoError(t, err)

		assert.Equal(t, changes(t, `{"title":"Scanner"}`), seen["plain"])
		assert.Equal(t, changes(t, `{"status":"paused","reason":"waiting"}`), seen["status"])
		title, status := row(t, db)
		assert.Equal(t, "Scanner", title)
		assert.Equal(t, "paused", status)
	})

	t.Run("rolls back every change when one fails", func(t *testing.T) {
		t.Parallel()
		db := openDatabase(t)

		err := tasks(db, map[string]rest.Changes{}).Update(t.Context(), "tkt_1",
			changes(t, `{"title":"Scanner","status":"closed"}`))
		assert.Equal(t, "invalid_transition", fault.CodeOf(err))

		title, status := row(t, db)
		assert.Equal(t, "Printer", title)
		assert.Equal(t, "open", status)
	})

	t.Run("leaves parameters without their field to the plain update", func(t *testing.T) {
		t.Parallel()
		seen := map[string]rest.Changes{}
		d := tasks(openDatabase(t), seen)
		d.Plain = func(_ context.Context, _ string, c rest.Changes) error {
			seen["plain"] = c

			return nil
		}

		require.NoError(t, d.Update(t.Context(), "tkt_1", changes(t, `{"reason":"waiting"}`)))
		assert.Equal(t, changes(t, `{"reason":"waiting"}`), seen["plain"])
		assert.NotContains(t, seen, "status")
	})

	t.Run("stops at a failing plain update", func(t *testing.T) {
		t.Parallel()
		seen := map[string]rest.Changes{}
		d := tasks(openDatabase(t), seen)

		err := d.Update(t.Context(), "tkt_1", changes(t, `{"title":7,"status":"paused"}`))
		require.Error(t, err)
		assert.NotContains(t, seen, "status")
	})

	t.Run("refuses fields nobody handles", func(t *testing.T) {
		t.Parallel()
		d := rest.Dispatcher[string]{Fields: map[string]rest.FieldHandler[string]{
			"status": func(context.Context, string, rest.Changes) error { return nil },
		}}

		err := d.Update(t.Context(), "tkt_1", changes(t, `{"title":"x","color":"red"}`))
		assert.Equal(t, map[string][]string{
			"color": {"The color field cannot be changed."},
			"title": {"The title field cannot be changed."},
		}, fields(t, err))
	})

	t.Run("runs without a transaction when given no database", func(t *testing.T) {
		t.Parallel()
		ran := errors.New("ran")
		d := rest.Dispatcher[string]{Fields: map[string]rest.FieldHandler[string]{
			"status": func(context.Context, string, rest.Changes) error { return ran },
		}}

		require.ErrorIs(t, d.Update(t.Context(), "tkt_1", changes(t, `{"status":"open"}`)), ran)
	})
}
