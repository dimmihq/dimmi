/*
|--------------------------------------------------------------------------
| Realtime Server
|--------------------------------------------------------------------------
|
| Every application node runs the same realtime server, so clients may
| connect to any of them and load balancers need no sticky sessions.
| Valkey carries broadcasts between nodes, keeps the history clients
| recover from after a reconnect, and tracks who is listening where.
| It holds nothing that cannot be lost: when it is flushed, clients
| reconnect, learn their history is gone and refetch what they show.
|
| The realtime connection to Valkey is built from the same options as
| the shared client, so it follows a single server, Sentinel or a
| cluster exactly as the rest of the application does.
|
*/

package broadcasting

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/centrifugal/centrifuge"
	"github.com/redis/rueidis"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/log"
	"github.com/dimmihq/dimmi/framework/redis"
)

/*
The paths clients connect to: centrifuge SDKs over WebSocket, and plain
EventSource clients over server-sent events, which only receive.
*/
const (
	PathWebsocket = "/connection/websocket"
	PathUniSSE    = "/connection/uni_sse"
)

/*
Defaults for every zero field of Config.
*/
const (
	DefaultKeyspace     = "rt"
	DefaultPingInterval = 25 * time.Second
	DefaultMaxPayload   = 10 << 10
	DefaultRecheck      = time.Minute
)

/*
ErrNoResolver stops Open without a resolver: a server that cannot tell
who connects would have to let everyone in or no one.
*/
var ErrNoResolver = errors.New("broadcasting: a resolver is required")

/*
Config configures the server.

Keyspace keeps realtime keys apart from the other services on the same
Valkey. PingInterval is how often the server pings each client, short
enough to keep idle connections open through proxies that close them
after a minute. MaxPayload caps an encoded broadcast in bytes. Recheck is
how often a connection is asked whether its credential is still alive,
which bounds how long one outlives a credential that ran out on its own.
Resolve authenticates each connection and is set in code, as is Clock,
which stamps broadcasts and defaults to the system clock.
*/
type Config struct {
	Keyspace     string        `default:"rt"    koanf:"keyspace"`
	PingInterval time.Duration `default:"25s"   koanf:"ping_interval"`
	MaxPayload   int           `default:"10240" koanf:"max_payload"`
	Recheck      time.Duration `default:"1m"    koanf:"recheck"`
	Resolve      Resolver      `koanf:"-"`
	Clock        clock.Clock   `koanf:"-"`
}

/*
Server is a running realtime node. It is safe for concurrent use.
*/
type Server struct {
	node       *centrifuge.Node
	shard      *centrifuge.RedisShard
	channels   *Channels
	resolve    Resolver
	logger     log.Logger
	clock      clock.Clock
	maxPayload int
	pingPong   centrifuge.PingPongConfig
	recheck    time.Duration
}

/*
Open starts a realtime node on the shared Valkey client's server. The
context supplies the logger and the fields of the node's own log lines.
Stop the node with Shutdown.
*/
func Open(ctx context.Context, client *redis.Client, cfg Config) (*Server, error) {
	if cfg.Resolve == nil {
		return nil, ErrNoResolver
	}

	ping := cmp.Or(cfg.PingInterval, DefaultPingInterval)
	s := &Server{
		channels:   &Channels{},
		resolve:    cfg.Resolve,
		logger:     log.From(ctx),
		clock:      cmp.Or(cfg.Clock, clock.System()),
		maxPayload: cmp.Or(cfg.MaxPayload, DefaultMaxPayload),
		pingPong:   centrifuge.PingPongConfig{PingInterval: ping, PongTimeout: ping * 2 / 5},
		recheck:    cmp.Or(cfg.Recheck, DefaultRecheck),
	}

	node, err := centrifuge.New(centrifuge.Config{
		LogLevel:   centrifuge.LogLevelInfo,
		LogHandler: logHandler(ctx, s.logger),
		UnidirectionalCodeToDisconnect: map[uint32]centrifuge.Disconnect{
			centrifuge.ErrorUnauthorized.Code: centrifuge.DisconnectInvalidToken,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create realtime node: %w", err)
	}

	s.node = node
	if err := s.useValkey(client, cmp.Or(cfg.Keyspace, DefaultKeyspace)); err != nil {
		return nil, err
	}

	node.OnConnecting(s.connecting)
	node.OnConnect(s.connected)
	node.OnNotification(s.noticed)

	if err := node.Run(); err != nil {
		s.shard.Close()

		return nil, fmt.Errorf("start realtime node: %w", err)
	}

	return s, nil
}

/*
useValkey connects the node's broker and presence manager to Valkey,
under the keyspace's prefix.
*/
func (s *Server) useValkey(client *redis.Client, keyspace string) error {
	shard, err := centrifuge.NewRedisShard(s.node, shardConfig(client.Option(), client.Rueidis().Mode()))
	if err != nil {
		return fmt.Errorf("connect realtime to valkey: %w", err)
	}

	prefix := strings.TrimSuffix(client.Prefix(keyspace), ":")
	shards := []*centrifuge.RedisShard{shard}

	broker, err := centrifuge.NewRedisBroker(s.node, centrifuge.RedisBrokerConfig{Prefix: prefix, Shards: shards})
	if err != nil {
		shard.Close()

		return fmt.Errorf("create realtime broker: %w", err)
	}

	presence, err := centrifuge.NewRedisPresenceManager(s.node, centrifuge.RedisPresenceManagerConfig{
		Prefix: prefix,
		Shards: shards,
	})
	if err != nil {
		shard.Close()

		return fmt.Errorf("create realtime presence: %w", err)
	}

	s.shard = shard
	s.node.SetBroker(broker)
	s.node.SetController(broker)
	s.node.SetPresenceManager(presence)

	return nil
}

/*
shardConfig rebuilds the shared client's connection for the realtime
node: the sentinels and the primary they watch, the cluster seeds, or
the one server, with the same credentials, TLS, database and time to
connect, so a slow server starts the node when it starts the client.
*/
func shardConfig(option rueidis.ClientOption, mode rueidis.ClientMode) centrifuge.RedisShardConfig {
	connect := option.Dialer.Timeout
	if connect == 0 {
		connect = rueidis.DefaultDialTimeout
	}

	cfg := centrifuge.RedisShardConfig{
		DB:             option.SelectDB,
		User:           option.Username,
		Password:       option.Password,
		ClientName:     option.ClientName,
		TLSConfig:      option.TLSConfig,
		ConnectTimeout: connect,
	}

	switch {
	case option.Sentinel.MasterSet != "":
		cfg.SentinelAddresses = option.InitAddress
		cfg.SentinelMasterName = option.Sentinel.MasterSet
		cfg.SentinelUser = option.Sentinel.Username
		cfg.SentinelPassword = option.Sentinel.Password
		cfg.SentinelClientName = option.Sentinel.ClientName
		cfg.SentinelTLSConfig = option.Sentinel.TLSConfig
	case mode == rueidis.ClientModeCluster:
		cfg.ClusterAddresses = option.InitAddress
	default:
		cfg.Address = option.InitAddress[0]
	}

	return cfg
}

/*
Channels returns the channel registry, where the application registers
its channel patterns and their authorizers.
*/
func (s *Server) Channels() *Channels {
	return s.channels
}

/*
Handlers returns the connection endpoints by path, for the router to
mount. Both answer GET requests only.
*/
func (s *Server) Handlers() map[string]http.Handler {
	websocket := centrifuge.NewWebsocketHandler(s.node, centrifuge.WebsocketConfig{PingPongConfig: s.pingPong})

	return map[string]http.Handler{
		PathWebsocket: withRequest(websocket),
		PathUniSSE:    withRequest(http.HandlerFunc(s.serveUniSSE)),
	}
}

/*
Shutdown disconnects every client of this node, telling them to
reconnect elsewhere, and closes its Valkey connections. History stays
in Valkey, so reconnecting clients recover what they missed.
*/
func (s *Server) Shutdown(ctx context.Context) error {
	err := s.node.Shutdown(ctx)
	s.shard.Close()

	if err != nil {
		return fmt.Errorf("shut down realtime node: %w", err)
	}

	return nil
}

/*
logHandler writes the node's log entries through the logger.
The node's levels, trace through error, are numbered one apart from
three for info; slog numbers its levels four apart from zero for info.
*/
func logHandler(ctx context.Context, logger log.Logger) centrifuge.LogHandler {
	return func(entry centrifuge.LogEntry) {
		attrs := make([]log.Attr, 0, len(entry.Fields)+2)
		attrs = append(attrs, log.String("detail", entry.Message))

		if entry.Error != nil {
			attrs = append(attrs, log.Err(entry.Error))
		}

		for key, value := range entry.Fields {
			attrs = append(attrs, log.Any(key, value))
		}

		logger.LogAttrs(ctx, slog.Level((int(entry.Level)-int(centrifuge.LogLevelInfo))*4), "realtime", attrs...)
	}
}
