/*
|--------------------------------------------------------------------------
| Security Log
|--------------------------------------------------------------------------
|
| Each user can read their own account's security log: sign-ins, failed
| and locked attempts, revoked sessions. Every entry records where it
| came from. Entries never hold a secret, only ids and methods.
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
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
)

/*
SecurityLog writes entries to users' security logs.
*/
type SecurityLog struct {
	DB      *database.DB
	Clock   clock.Clock
	Devices *Devices
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
Record writes the entry, inside the transaction in the context when
there is one. Its details hold only strings, numbers, booleans and
lists of them, which always encode, so that error is not checked.
*/
func (l *SecurityLog) Record(ctx context.Context, e Entry) error {
	metadata, _ := json.Marshal(e.Metadata)

	err := models.New(l.DB).CreateSecurityEvent(ctx, models.CreateSecurityEventParams{
		UserID:      e.UserID,
		Type:        e.Type,
		Ip:          address(e.Client.IP),
		UserAgent:   optional(e.Client.TruncatedUserAgent()),
		DeviceLabel: optional(l.Devices.Label(e.Client.UserAgent)),
		Metadata:    metadata,
		Now:         l.Clock.Now(),
	})
	if err != nil {
		return fmt.Errorf("record security event %s: %w", e.Type, err)
	}

	return nil
}

/*
address returns the client's address as stored, or nil when it is not
one.
*/
func address(ip string) *netip.Addr {
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
