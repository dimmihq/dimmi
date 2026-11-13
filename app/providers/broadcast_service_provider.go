/*
|--------------------------------------------------------------------------
| Broadcast Service Provider
|--------------------------------------------------------------------------
|
| Keeps open screens current. Realtime connections are authenticated by
| the API's own guards, and a browser's cookie only counts when the
| socket was opened from the same origins cross-origin protection
| trusts for the API. Domain events become broadcasts once they commit,
| connections end with the access or credential behind them, and
| teammates typing and reading are relayed to the customer's side.
|
*/

package providers

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/foundation"
	"github.com/dimmihq/dimmi/framework/http/middleware"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/conversations"
	appauth "github.com/dimmihq/dimmi/app/auth"
	appbroadcasting "github.com/dimmihq/dimmi/app/broadcasting"
	"github.com/dimmihq/dimmi/app/http/resources"
)

/*
BroadcastServiceProvider wires realtime connections, broadcasts and
the typing relay.
*/
type BroadcastServiceProvider struct {
	App *app.App
}

/*
Register builds the connection resolver and makes the broadcaster hear
the events it broadcasts.
*/
func (p BroadcastServiceProvider) Register(fa *foundation.Application) error {
	trusted := append([]string{origin(fa.Config.App.URL)}, p.App.Config.HTTP.TrustedOrigins...)

	origins, err := middleware.NewOriginCheck(trusted...)
	if err != nil {
		return fmt.Errorf("realtime origin check: %w", err)
	}

	p.App.Realtime = &appbroadcasting.Connections{
		Guards: fa.Guards, Sessions: fa.Session, Origins: origins,
		Live: &appauth.LiveCredentials{DB: fa.DB, Clock: fa.Clock, Sessions: fa.Session},
	}

	p.App.Viewers = &appbroadcasting.Viewers{Presence: fa.Broadcast, DB: fa.DB, IDs: p.App.IDs}

	broadcaster := &appbroadcasting.Broadcaster{
		Server:      fa.Broadcast,
		Payloads:    resources.BroadcastPayloads{DB: fa.DB, Disks: fa.Storage},
		Unsubscribe: fa.Broadcast,
		Fanout:      &appbroadcasting.Fanout{DB: fa.DB, Cache: fa.Cache},
	}
	broadcaster.Listen(fa.Events)

	disconnects := &appbroadcasting.Disconnects{Server: fa.Broadcast, DB: fa.DB}
	disconnects.Listen(fa.Events)

	seen := &appbroadcasting.SeenReceipts{DB: fa.DB, Cache: fa.Cache, Server: fa.Broadcast}
	seen.Listen(fa.Events)

	return nil
}

/*
Boot builds the typing relay.
*/
func (p BroadcastServiceProvider) Boot(context.Context, *foundation.Application) error {
	a := p.App
	a.Actions.RecordTyping = &conversations.RecordTyping{
		DB: a.DB, Cache: a.Cache, Clock: a.Clock, Publisher: a.Broadcast,
	}

	return nil
}
