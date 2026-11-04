/*
|--------------------------------------------------------------------------
| Presence
|--------------------------------------------------------------------------
|
| Who is listening on a channel, read from Valkey so it counts every
| node. A list of many channels asks for all of them at once: the Valkey
| client pipelines commands issued together on one connection, so a page
| of channels costs about one round trip, not one each. Whoever waits for
| it can give up at their deadline; what was still in flight finishes on
| its own and is dropped.
|
*/

package broadcasting

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"
)

/*
Presence returns the connections listening on the channel, across every
node, ordered by user and then client.
*/
func (s *Server) Presence(ctx context.Context, channel string) ([]Member, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("presence on %q: %w", channel, err)
	}

	return s.readPresence(channel)
}

/*
PresenceMany returns the connections listening on each of the channels,
by channel. A channel nobody listens on has an empty list. It answers
with the context's error as soon as the context ends, and with the first
error any channel's read returns: a partial answer would look like
people having left.
*/
func (s *Server) PresenceMany(ctx context.Context, channels []string) (map[string][]Member, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("presence on %d channels: %w", len(channels), err)
	}

	type reply struct {
		channel string
		members []Member
		err     error
	}

	replies := make(chan reply, len(channels))

	for _, channel := range channels {
		go func() {
			members, err := s.readPresence(channel)
			replies <- reply{channel: channel, members: members, err: err}
		}()
	}

	out := make(map[string][]Member, len(channels))

	for range channels {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("presence on %d channels: %w", len(channels), ctx.Err())
		case r := <-replies:
			if r.err != nil {
				return nil, r.err
			}

			out[r.channel] = r.members
		}
	}

	return out, nil
}

/*
readPresence reads one channel's listeners from Valkey.
*/
func (s *Server) readPresence(channel string) ([]Member, error) {
	result, err := s.node.Presence(channel)
	if err != nil {
		return nil, fmt.Errorf("presence on %q: %w", channel, err)
	}

	members := make([]Member, 0, len(result.Presence))
	for _, info := range result.Presence {
		members = append(members, Member{ClientID: info.ClientID, UserID: info.UserID, Info: info.ConnInfo})
	}

	slices.SortFunc(members, func(a, b Member) int {
		return cmp.Or(strings.Compare(a.UserID, b.UserID), strings.Compare(a.ClientID, b.ClientID))
	})

	return members, nil
}
