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
| twice stores one row, not two. Rows are only ever marked read for the
| recipient they belong to.
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
	insert, markRead, markUnread, markAllRead string
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
		insert: `INSERT INTO ` + table + ` (id, notifiable_type, notifiable_id, type, data, created_at)
VALUES ($1, $2, $3, $4, $5::jsonb, $6) ON CONFLICT (id) DO NOTHING`,
		markRead: `UPDATE ` + table + ` SET read_at = $3
WHERE notifiable_type = $1 AND notifiable_id = $2 AND id = ANY($4::uuid[]) AND read_at IS NULL`,
		markUnread: `UPDATE ` + table + ` SET read_at = NULL
WHERE notifiable_type = $1 AND notifiable_id = $2 AND id = ANY($3::uuid[])`,
		markAllRead: `UPDATE ` + table + ` SET read_at = $3
WHERE notifiable_type = $1 AND notifiable_id = $2 AND read_at IS NULL`,
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

	to := d.Notifiable
	if _, err := c.db.Exec(ctx, c.sql.insert, d.ID, to.NotifiableType(), to.NotifiableID(), n.Type(),
		string(encoded), c.clock.Now()); err != nil {
		return fmt.Errorf("store %s: %w", n.Type(), err)
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
quote checks the table name and quotes it for SQL.
*/
func quote(table string) (string, error) {
	if !tableName.MatchString(table) {
		return "", fmt.Errorf("%w: %q", ErrInvalidTable, table)
	}

	return pgx.Identifier{table}.Sanitize(), nil
}
