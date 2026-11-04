/*
|--------------------------------------------------------------------------
| Widget Directory
|--------------------------------------------------------------------------
|
| Every visitor request starts by finding its widget, and a page load must
| stay in the low milliseconds, so each replica keeps widgets in memory for
| a minute. Any change to a widget tells every replica to drop its copy at
| once, which is why a change to allowed sites applies immediately, while a
| replica that misses the notice still catches up when the minute is over.
|
| Finding a widget is the one read that happens before a workspace is known:
| a resolver function names it, and everything after is read inside it.
|
*/

package widget

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/log"
	"github.com/dimmihq/dimmi/framework/memo"
	"github.com/dimmihq/dimmi/framework/pubsub"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
directoryTopic names the notice that a widget changed.
*/
const directoryTopic = "widget.changed"

/*
maxCachedWidgets and maxCachedConfigs bound what a replica keeps in memory.
*/
const (
	maxCachedWidgets = 50_000
	maxCachedConfigs = 100_000
)

/*
Resolved is a widget with what a visitor's request needs beside it: its
workspace and the inbox its new conversations land in.
*/
type Resolved struct {
	Widget    models.WidgetSetting
	Workspace models.Workspace
	InboxID   ids.UUID
}

/*
Directory finds widgets and caches what is built from them.
*/
type Directory struct {
	db      *database.DB
	bus     *pubsub.Bus
	widgets *memo.Cache[ids.UUID, Resolved]
	configs *memo.Cache[string, Config]
}

/*
NewDirectory returns a directory keeping widgets for the time given.
*/
func NewDirectory(db *database.DB, bus *pubsub.Bus, ttl time.Duration) *Directory {
	return &Directory{
		db: db, bus: bus,
		widgets: memo.New[ids.UUID, Resolved](memo.Options{Size: maxCachedWidgets, TTL: ttl}),
		configs: memo.New[string, Config](memo.Options{Size: maxCachedConfigs, TTL: ttl}),
	}
}

/*
Listen starts hearing other replicas' notices, and returns how to stop.
*/
func (d *Directory) Listen(ctx context.Context) (pubsub.Unsubscribe, error) {
	unsubscribe, err := d.bus.Subscribe(ctx, directoryTopic, func(payload []byte) {
		if id, err := ids.Parse(string(payload)); err == nil {
			d.drop(id)
		}
	})
	if err != nil {
		return nil, fmt.Errorf("listen for widget changes: %w", err)
	}

	return unsubscribe, nil
}

/*
Forget drops the widget here and tells every other replica to. A lost notice
only delays the change until the copies expire, so it is logged and nothing
is undone.
*/
func (d *Directory) Forget(ctx context.Context, id ids.UUID) {
	d.drop(id)

	if err := d.bus.Publish(ctx, directoryTopic, []byte(id.String())); err != nil {
		log.From(ctx).WarnContext(ctx, "publish widget change", log.Err(err))
	}
}

/*
drop removes the widget from this replica.
*/
func (d *Directory) drop(id ids.UUID) {
	d.widgets.Delete(id)
}

/*
Find returns the live widget with the id, from memory when it can, and
[exceptions.ErrAppNotFound] when there is none.
*/
func (d *Directory) Find(ctx context.Context, id ids.UUID) (Resolved, error) {
	if found, ok := d.widgets.Get(id); ok {
		return found, nil
	}

	resolved, err := d.load(ctx, id)
	if err != nil {
		return resolved, err
	}

	d.widgets.Set(id, resolved)

	return resolved, nil
}

/*
load reads the widget through its resolver, then inside its workspace.
*/
func (d *Directory) load(ctx context.Context, id ids.UUID) (Resolved, error) {
	var out Resolved

	row, err := models.New(d.db).ResolveWidgetApp(ctx, id)
	if errors.Is(err, database.ErrNotFound) {
		return out, exceptions.ErrAppNotFound
	}

	if err != nil {
		return out, fmt.Errorf("resolve widget: %w", err)
	}

	scoped := tenancy.With(ctx, row.WorkspaceID.String())

	err = d.db.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		out.Workspace, err = q.FindWorkspace(ctx, row.WorkspaceID)
		if err != nil {
			return fmt.Errorf("find workspace: %w", err)
		}

		out.Widget, err = q.FindWidgetSettings(ctx, models.FindWidgetSettingsParams{
			WorkspaceID: row.WorkspaceID, ID: id,
		})
		if err != nil {
			return fmt.Errorf("find widget: %w", err)
		}

		conn, err := q.FindChannelConnection(ctx, models.FindChannelConnectionParams{
			WorkspaceID: row.WorkspaceID, ID: out.Widget.ChannelConnectionID,
		})
		if err != nil {
			return fmt.Errorf("find widget channel: %w", err)
		}

		out.InboxID = conn.InboxID

		return nil
	})

	return out, err
}

/*
config returns the config cached under the key.
*/
func (d *Directory) config(key string) (Config, bool) {
	return d.configs.Get(key)
}

/*
keepConfig caches a built config under the key.
*/
func (d *Directory) keepConfig(key string, config Config) {
	d.configs.Set(key, config)
}
