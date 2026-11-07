/*
|--------------------------------------------------------------------------
| Database Channel
|--------------------------------------------------------------------------
|
| A notification stored as a row stays until the recipient reads it,
| such as in a list behind a bell. The row keeps the notification type
| and the data it rendered for this channel, so it can be shown long
| after the code that sent it has changed.
|
| The row is keyed by the delivery id, so a queued delivery that runs
| twice stores one row, not two. A grouped notification, such as every
| new reply in one thread, folds into the recipient's unread row
| of its group instead: the row counts the occurrences, carries the
| latest data and becomes unseen again, so a busy thread is one line in
| the list, not fifty. Rows belong to a tenant when the notification
| names one, and are written in that tenant's scope. Rows are only ever
| marked read or seen for the recipient they belong to.
|
*/

package database

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"

	"github.com/jackc/pgx/v5"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/manager"
	"github.com/dimmihq/dimmi/framework/notifications"
	"github.com/dimmihq/dimmi/framework/tenancy"
)

/*
ErrInvalidTable refuses a table name that is not a plain lowercase SQL
identifier of at most 45 characters, which leaves room for the names
derived from it.
*/
var ErrInvalidTable = errors.New("notifications database: invalid table name")

/*
tableName is a plain lowercase SQL identifier.
*/
var tableName = regexp.MustCompile(`^[a-z_][a-z0-9_]{0,44}$`)

/*
Config names the table notifications are stored in.
*/
type Config struct {
	Table string `default:"notifications" koanf:"table"`
}

/*
Notification is a notification that can be stored as a row.
*/
type Notification interface {
	/*
		ToDatabase returns the data stored for the delivery, encoded as
		JSON.
	*/
	ToDatabase(ctx context.Context, d notifications.Delivery) (any, error)
}

/*
Tenanted is a notification that belongs to a tenant; its rows carry
the tenant and are written in its scope.
*/
type Tenanted interface {
	/*
		TenantID returns the tenant the notification belongs to.
	*/
	TenantID() string
}

/*
Grouped is a notification whose occurrences fold into one unread row
per recipient and group.
*/
type Grouped interface {
	/*
		GroupKey returns the group of the delivery, "" for none.
	*/
	GroupKey(d notifications.Delivery) string
}

/*
Channel stores notifications as rows and marks them read.
*/
type Channel struct {
	db    database.Querier
	clock clock.Clock
	sql   statements
}

/*
statements are the channel's SQL, written for its table.
*/
type statements struct {
	insert, group, markRead, markUnread, markAllRead, markSeen string
}

/*
Factory builds database channels for the notification manager.
*/
func Factory(db database.Querier, clk clock.Clock) manager.Factory[notifications.Channel] {
	return func(_ string, section config.Section) (notifications.Channel, error) {
		var cfg Config
		if err := section.Decode(&cfg); err != nil {
			return nil, fmt.Errorf("notifications database: %w", err)
		}

		return New(db, clk, cfg)
	}
}

/*
New returns a channel storing rows in the configured table, dating them
by the clock. Queries made through it join the transaction carried by
their context.
*/
func New(db database.Querier, clk clock.Clock, cfg Config) (*Channel, error) {
	table, err := quote(cfg.Table)
	if err != nil {
		return nil, err
	}

	return &Channel{db: db, clock: clk, sql: statements{
		insert: `INSERT INTO ` + table + ` (id, notifiable_type, notifiable_id, type, data, tenant_id,
	last_delivery_id, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5::jsonb, $6, $8, $7, $7) ON CONFLICT (id) DO NOTHING`,
		group: `INSERT INTO ` + table + ` AS n (id, notifiable_type, notifiable_id, type, data, tenant_id, group_key,
	last_delivery_id, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5::jsonb, $6, $9, $8, $7, $7)
ON CONFLICT (notifiable_type, notifiable_id, group_key) WHERE read_at IS NULL AND group_key IS NOT NULL
DO UPDATE SET count = n.count + 1, data = excluded.data, updated_at = excluded.updated_at, seen_at = NULL,
	last_delivery_id = excluded.last_delivery_id
WHERE n.last_delivery_id IS DISTINCT FROM excluded.last_delivery_id`,
		markRead: `UPDATE ` + table + ` SET read_at = $3
WHERE notifiable_type = $1 AND notifiable_id = $2 AND id = ANY($4::uuid[]) AND read_at IS NULL`,
		markUnread: `UPDATE ` + table + ` SET read_at = NULL
WHERE notifiable_type = $1 AND notifiable_id = $2 AND id = ANY($3::uuid[])`,
		markAllRead: `UPDATE ` + table + ` SET read_at = $3
WHERE notifiable_type = $1 AND notifiable_id = $2 AND read_at IS NULL`,
		markSeen: `UPDATE ` + table + ` SET seen_at = $3
WHERE notifiable_type = $1 AND notifiable_id = $2 AND seen_at IS NULL AND updated_at <= $3`,
	}}, nil
}

/*
Send stores the notification's data as a row for the recipient.
*/
func (c *Channel) Send(ctx context.Context, d notifications.Delivery, n notifications.Notification) error {
	renderer, ok := n.(Notification)
	if !ok {
		return fmt.Errorf("store %s: %w", n.Type(), notifications.ErrUnsupported)
	}

	data, err := renderer.ToDatabase(ctx, d)
	if err != nil {
		return fmt.Errorf("render %s for database: %w", n.Type(), err)
	}

	encoded, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("encode %s: %w: %w", n.Type(), notifications.ErrPermanent, err)
	}

	to, tenant, group := d.Notifiable, tenantOf(n), groupOf(n, d)
	args := []any{
		d.ID, to.NotifiableType(), to.NotifiableID(), n.Type(), string(encoded), tenant, c.clock.Now(), d.ID,
	}
	statement := c.sql.insert
	if group != nil {
		statement, args = c.sql.group, append(args, *group)
	}

	if err := c.exec(ctx, tenant, statement, args...); err != nil {
		return fmt.Errorf("store %s: %w", n.Type(), err)
	}

	return nil
}

/*
transactor runs work in a transaction that carries the context's
tenant, which *database.DB does.
*/
type transactor interface {
	Transaction(ctx context.Context, fn func(ctx context.Context, tx *database.Tx) error) error
}

/*
exec runs a write, inside the tenant's scope when the notification
names one and the database can open a transaction for it.
*/
func (c *Channel) exec(ctx context.Context, tenant *string, statement string, args ...any) error {
	db, ok := c.db.(transactor)
	if tenant == nil || !ok {
		_, err := c.db.Exec(ctx, statement, args...)
		return err
	}

	return db.Transaction(tenancy.With(ctx, *tenant), func(ctx context.Context, _ *database.Tx) error {
		_, err := c.db.Exec(ctx, statement, args...)
		return err
	})
}

/*
tenantOf returns the notification's tenant, nil when it names none.
*/
func tenantOf(n notifications.Notification) *string {
	if t, ok := n.(Tenanted); ok && t.TenantID() != "" {
		id := t.TenantID()
		return &id
	}

	return nil
}

/*
groupOf returns the delivery's group, nil when it has none.
*/
func groupOf(n notifications.Notification, d notifications.Delivery) *string {
	if g, ok := n.(Grouped); ok && g.GroupKey(d) != "" {
		key := g.GroupKey(d)
		return &key
	}

	return nil
}

/*
MarkRead marks the recipient's notifications with the ids read. Ids of
other recipients' notifications are ignored.
*/
func (c *Channel) MarkRead(ctx context.Context, to notifications.Notifiable, ids ...string) error {
	_, err := c.db.Exec(ctx, c.sql.markRead, to.NotifiableType(), to.NotifiableID(), c.clock.Now(), ids)
	if err != nil {
		return fmt.Errorf("mark notifications read: %w", err)
	}

	return nil
}

/*
MarkUnread marks the recipient's notifications with the ids unread
again.
*/
func (c *Channel) MarkUnread(ctx context.Context, to notifications.Notifiable, ids ...string) error {
	_, err := c.db.Exec(ctx, c.sql.markUnread, to.NotifiableType(), to.NotifiableID(), ids)
	if err != nil {
		return fmt.Errorf("mark notifications unread: %w", err)
	}

	return nil
}

/*
MarkAllRead marks every unread notification of the recipient read.
*/
func (c *Channel) MarkAllRead(ctx context.Context, to notifications.Notifiable) error {
	_, err := c.db.Exec(ctx, c.sql.markAllRead, to.NotifiableType(), to.NotifiableID(), c.clock.Now())
	if err != nil {
		return fmt.Errorf("mark all notifications read: %w", err)
	}

	return nil
}

/*
MarkSeen marks every notification of the recipient that changed by now
seen, such as when they open the list behind a bell.
*/
func (c *Channel) MarkSeen(ctx context.Context, to notifications.Notifiable) error {
	_, err := c.db.Exec(ctx, c.sql.markSeen, to.NotifiableType(), to.NotifiableID(), c.clock.Now())
	if err != nil {
		return fmt.Errorf("mark notifications seen: %w", err)
	}

	return nil
}

/*
quote checks the table name and quotes it for SQL.
*/
func quote(table string) (string, error) {
	if !tableName.MatchString(table) {
		return "", fmt.Errorf("%w: %q", ErrInvalidTable, table)
	}

	return pgx.Identifier{table}.Sanitize(), nil
}
