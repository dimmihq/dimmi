/*
|--------------------------------------------------------------------------
| Connection Limits
|--------------------------------------------------------------------------
|
| A user may keep only so many connections open, one per browser tab, so
| a page that leaks tabs cannot pile streams onto the servers. The newest
| connection always wins: once a user is over the limit, the oldest close
| with their own code, and the client behind them knows to fall back to
| asking for news rather than reconnecting.
|
| Connections are counted on every node at once. Each capped connection
| joins a presence list of its own user, stamped with when it connected,
| so whichever node took the newest one sees all of them.
|
*/

package broadcasting

import (
	"cmp"
	"context"
	"encoding/json"
	"slices"
	"strings"

	"github.com/centrifugal/centrifuge"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/log"
)

/*
connectionsPrefix starts the presence list of a user's capped
connections. No application channel pattern starts this way.
*/
const connectionsPrefix = "_connections:"

/*
connectedStamp is what a capped connection records about itself in its
user's presence list.
*/
type connectedStamp struct {
	At int64 `json:"at"`
}

/*
limited is the disconnect of a connection closed to make room for a
newer one of the same user. The client must not reconnect on its own.
*/
func limited() centrifuge.Disconnect {
	return centrifuge.Disconnect{Code: 4004, Reason: "connection_limit"}
}

/*
connectionsChannel names the presence list of the user's capped
connections.
*/
func connectionsChannel(userID string) string {
	return connectionsPrefix + userID
}

/*
countConnection adds the presence list to a capped connection's
subscriptions, stamped with the wall-clock time so the oldest is known
across nodes.
*/
func countConnection(principal Principal, subscriptions map[string]centrifuge.SubscribeOptions) error {
	if principal.MaxConnections <= 0 {
		return nil
	}

	stamp, err := json.Marshal(connectedStamp{At: clock.System().Now().UnixNano()})
	if err != nil {
		return err
	}

	subscriptions[connectionsChannel(principal.UserID)] = centrifuge.SubscribeOptions{
		EmitPresence: true, ChannelInfo: stamp,
	}

	return nil
}

/*
enforceLimit closes the user's oldest connections beyond the principal's
limit. A presence read that fails leaves everyone connected.
*/
func (s *Server) enforceLimit(ctx context.Context, principal Principal) {
	if principal.MaxConnections <= 0 {
		return
	}

	result, err := s.node.Presence(connectionsChannel(principal.UserID))
	if err != nil {
		s.logger.ErrorContext(ctx, "count realtime connections", log.Err(err))

		return
	}

	excess := len(result.Presence) - principal.MaxConnections
	if excess <= 0 {
		return
	}

	for _, id := range oldest(result.Presence, excess) {
		err := s.node.Disconnect(principal.UserID,
			centrifuge.WithDisconnectClient(id), centrifuge.WithCustomDisconnect(limited()))
		if err != nil {
			s.logger.ErrorContext(ctx, "close realtime connection over limit", log.Err(err))
		}
	}
}

/*
oldest returns the ids of the n connections that connected first, the
client id breaking ties.
*/
func oldest(presence map[string]*centrifuge.ClientInfo, n int) []string {
	type entry struct {
		id string
		at int64
	}

	entries := make([]entry, 0, len(presence))
	for id, info := range presence {
		var stamp connectedStamp
		_ = json.Unmarshal(info.ChanInfo, &stamp)
		entries = append(entries, entry{id: id, at: stamp.At})
	}

	slices.SortFunc(entries, func(a, b entry) int {
		return cmp.Or(cmp.Compare(a.at, b.at), strings.Compare(a.id, b.id))
	})

	ids := make([]string, 0, n)
	for _, e := range entries[:n] {
		ids = append(ids, e.id)
	}

	return ids
}
