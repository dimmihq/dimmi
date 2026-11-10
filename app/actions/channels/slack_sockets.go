/*
|--------------------------------------------------------------------------
| Slack Sockets
|--------------------------------------------------------------------------
|
| A workspace's own app on a socket needs exactly one open socket across
| every replica, or each event would arrive once per replica. Every few
| seconds each worker takes the lease of every socket nobody holds, or
| whose holder stopped renewing, and renews the ones it holds; a socket
| whose lease it loses is closed at once. A replica that dies stops
| renewing, and another takes its sockets over within the lease's life.
|
| Deliveries down a socket are kept exactly as webhook deliveries are,
| for the socket's own connection, and acknowledged only once kept.
|
*/

package channels

import (
	"context"
	"sync"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/log"
	"github.com/dimmihq/dimmi/framework/tenancy"

	slackdriver "github.com/dimmihq/dimmi/app/drivers/channel/slack"
	"github.com/dimmihq/dimmi/app/models"
)

/*
socketKind names the lease kind of a Slack socket.
*/
const socketKind = "slack_socket"

/*
SocketSupervisor holds the Slack sockets this replica leases. Owner
names the replica.
*/
type SocketSupervisor struct {
	Slack *Slack
	Owner string

	mu      sync.Mutex
	running map[ids.UUID]context.CancelFunc
}

/*
Run leases and runs sockets until the context ends.
*/
func (sv *SocketSupervisor) Run(ctx context.Context) {
	ticker := sv.Slack.Clock.NewTicker(sv.Slack.Config.RenewInterval)
	defer ticker.Stop()

	for {
		sv.Tick(ctx)

		select {
		case <-ctx.Done():
			sv.stopAll()

			return
		case <-ticker.Chan():
		}
	}
}

/*
Tick takes and renews the leases once: every socket connection whose
lease this replica holds runs, every other is closed here.
*/
func (sv *SocketSupervisor) Tick(ctx context.Context) {
	q := models.New(sv.Slack.DB)

	wanted, err := q.ListSocketChannelConnections(ctx, slackdriver.Type)
	if err != nil {
		return
	}

	held := map[ids.UUID]bool{}

	for _, row := range wanted {
		if sv.lease(ctx, q, row.ChannelConnectionID) {
			held[row.ChannelConnectionID] = true
			sv.start(ctx, row.WorkspaceID, row.ChannelConnectionID)
		}
	}

	sv.mu.Lock()
	defer sv.mu.Unlock()

	for id, stop := range sv.running {
		if !held[id] {
			stop()
			delete(sv.running, id)
		}
	}
}

/*
lease takes or renews the socket's lease, reporting whether this
replica holds it.
*/
func (sv *SocketSupervisor) lease(ctx context.Context, q *models.Queries, connectionID ids.UUID) bool {
	now := sv.Slack.Clock.Now()
	key := connectionID.String()

	if err := q.EnsureChannelSocketSession(ctx, models.EnsureChannelSocketSessionParams{
		Kind: socketKind, Key: key, Now: now,
	}); err != nil {
		return false
	}

	_, err := q.AcquireChannelSocketLease(ctx, models.AcquireChannelSocketLeaseParams{
		Owner: &sv.Owner, ExpiresAt: now.Add(sv.Slack.Config.LeaseTTL), Now: now, Kind: socketKind, Key: key,
	})

	return err == nil
}

/*
start runs the socket of the connection unless it already runs here.
*/
func (sv *SocketSupervisor) start(ctx context.Context, workspaceID, connectionID ids.UUID) {
	sv.mu.Lock()
	defer sv.mu.Unlock()

	if sv.running == nil {
		sv.running = map[ids.UUID]context.CancelFunc{}
	}

	if _, ok := sv.running[connectionID]; ok {
		return
	}

	socketCtx, stop := context.WithCancel(tenancy.With(ctx, workspaceID.String()))
	sv.running[connectionID] = stop

	go sv.hold(socketCtx, routed{workspaceID: workspaceID, connectionID: connectionID})
}

/*
hold holds the connection's socket open until it ends, then gives the
lease up so the next tick, here or elsewhere, opens it again.
*/
func (sv *SocketSupervisor) hold(ctx context.Context, r routed) {
	err := sv.socket(ctx, r)

	sv.mu.Lock()
	delete(sv.running, r.connectionID)
	sv.mu.Unlock()

	var reason *string
	if err != nil {
		reason = new(err.Error())
		sv.Slack.Log.WarnContext(ctx, "slack socket ended", log.String("connection_id", r.connectionID.String()),
			log.Err(err))
	}

	_ = models.New(sv.Slack.DB).ReleaseChannelSocketLease(context.WithoutCancel(ctx),
		models.ReleaseChannelSocketLeaseParams{
			LastError: reason, Now: sv.Slack.Clock.Now(), Kind: socketKind, Key: r.connectionID.String(),
			Owner: &sv.Owner,
		})
}

/*
socket opens the connection's socket and keeps what it delivers.
*/
func (sv *SocketSupervisor) socket(ctx context.Context, r routed) error {
	conn, err := sv.Slack.connection(ctx, r.workspaceID, r.connectionID)
	if err != nil {
		return err
	}

	secrets, err := sv.Slack.Secrets.Open(ctx, conn)
	if err != nil {
		return err
	}

	opts := slackdriver.SocketOptions{
		AppToken: secrets.AppToken, BotToken: secrets.BotToken, HTTP: sv.Slack.Driver.HTTP,
		APIURL: sv.Slack.Driver.APIURL,
	}

	return slackdriver.RunSocket(ctx, opts, func(ctx context.Context, kind string, payload []byte) error {
		return sv.Slack.keepSocket(ctx, r, kind, payload)
	})
}

/*
keepSocket keeps one socket delivery for its connection.
*/
func (s *Slack) keepSocket(ctx context.Context, r routed, kind string, payload []byte) error {
	if kind == slackdriver.SocketInteraction {
		in, ok := readInteraction(payload)
		if !ok {
			return nil
		}

		return s.keepInteraction(ctx, &r, in, payload)
	}

	env, ok := readEnvelope(payload)
	if !ok {
		return nil
	}

	return s.keepEvent(ctx, &r, eventDelivery{env: env, raw: payload, transport: TransportSocket})
}

/*
stopAll closes every socket this replica runs.
*/
func (sv *SocketSupervisor) stopAll() {
	sv.mu.Lock()
	defer sv.mu.Unlock()

	for id, stop := range sv.running {
		stop()
		delete(sv.running, id)
	}
}
