/*
|--------------------------------------------------------------------------
| Security Log
|--------------------------------------------------------------------------
|
| Each user can read their own account's security log: sign-ins, failed
| and locked attempts, revoked sessions. Every entry records where it
| came from. Entries never hold a secret, only ids and methods. Each one
| is announced in the transaction that wrote it, so listeners can carry
| it further once it is saved.
|
*/

package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"

	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
SecurityLog writes entries to users' security logs and dispatches
SecurityEventRecorded for each through Events.
*/
type SecurityLog struct {
	DB      *database.DB
	Clock   clock.Clock
	Devices *Devices
	Events  *events.Dispatcher
}

/*
Entry is one security log entry: what happened to which user, from
which client, with details such as the methods used.
*/
type Entry struct {
	UserID   ids.UUID
	Type     string
	Client   Client
	Metadata map[string]any
}

/*
Record writes the entry and announces it, inside the transaction in the
context when there is one, else in one of its own. Its details hold
only strings, numbers, booleans and lists of them, which always encode,
so that error is not checked.
*/
func (l *SecurityLog) Record(ctx context.Context, e Entry) error {
	metadata, _ := json.Marshal(e.Metadata)
	recorded := appevents.SecurityEventRecorded{
		UserID: e.UserID, Type: e.Type, IP: e.Client.IP, UserAgent: e.Client.TruncatedUserAgent(),
		DeviceLabel: l.Devices.Label(e.Client.UserAgent), Metadata: e.Metadata, At: l.Clock.Now(),
	}

	return l.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		err := models.New(tx).CreateSecurityEvent(ctx, models.CreateSecurityEventParams{
			UserID:      e.UserID,
			Type:        e.Type,
			Ip:          Address(e.Client.IP),
			UserAgent:   optional(recorded.UserAgent),
			DeviceLabel: optional(recorded.DeviceLabel),
			Metadata:    metadata,
			Now:         recorded.At,
		})
		if err != nil {
			return fmt.Errorf("record security event %s: %w", e.Type, err)
		}

		return l.Events.Dispatch(ctx, tx, recorded)
	})
}

/*
Address returns a client's address as stored, or nil when it is not
one.
*/
func Address(ip string) *netip.Addr {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return nil
	}

	return &addr
}

/*
optional returns nil for the zero value, which the table stores as
NULL.
*/
func optional(s string) *string {
	if s == "" {
		return nil
	}

	return &s
}
