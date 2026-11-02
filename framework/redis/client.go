/*
|--------------------------------------------------------------------------
| Valkey Client
|--------------------------------------------------------------------------
|
| Valkey holds only state we can rebuild: caches, counters, locks, pub/sub
| and realtime history. Operators may run it as one server, behind Sentinel
| or as a cluster, and the application must work the same on all three
| without code changes. That is why keys are built in one place: keys that
| share a tag share a cluster slot, so multi-key work never fails there.
|
*/

package redis

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/redis/rueidis"
)

/*
Mode is the deployment shape of the Valkey server.

	ModeAuto        detect a cluster at connect time, else one server
	ModeStandalone  one server, or a managed endpoint fronting one primary
	ModeSentinel    follow the primary that Sentinel elects
	ModeCluster     require a cluster
*/
type Mode string

/*
The supported modes, described on Mode.
*/
const (
	ModeAuto       Mode = "auto"
	ModeStandalone Mode = "standalone"
	ModeSentinel   Mode = "sentinel"
	ModeCluster    Mode = "cluster"
)

/*
Configuration errors, returned by Open.

A cluster has only database 0, so a cluster URL that selects another
database is a mistake worth stopping the boot for, and so is cluster mode
pointed at a single server.
*/
var (
	ErrInvalidMode     = errors.New("redis: invalid mode")
	ErrSentinelMaster  = errors.New("redis: sentinel mode needs sentinel.master")
	ErrClusterDatabase = errors.New("redis: logical databases are unsupported in cluster mode")
	ErrNotCluster      = errors.New("redis: cluster mode is set but the server is not a cluster")
)

/*
Config describes how to reach Valkey.

The URL carries address, credentials, TLS (rediss://) and database. Addrs
adds cluster seed nodes, or lists the sentinels in sentinel mode. Every
key starts with KeyPrefix, so several applications can share a server;
the application chooses it and ends it with a colon.
*/
type Config struct {
	URL       string         `koanf:"url"`
	Mode      Mode           `default:"auto" koanf:"mode"`
	Addrs     []string       `koanf:"addrs"`
	Sentinel  SentinelConfig `koanf:"sentinel"`
	KeyPrefix string         `koanf:"key_prefix"`
}

/*
SentinelConfig names the primary the sentinels watch.
*/
type SentinelConfig struct {
	Master string `koanf:"master"`
}

/*
Client is a connection to Valkey.
*/
type Client struct {
	client rueidis.Client
	option rueidis.ClientOption
	prefix string
}

/*
Open connects to Valkey and checks that it answers.
*/
func Open(ctx context.Context, cfg Config) (*Client, error) {
	option, err := clientOption(cfg)
	if err != nil {
		return nil, err
	}

	client, err := rueidis.NewClient(option)
	if err != nil {
		return nil, fmt.Errorf("connect to valkey: %w", err)
	}

	if err := checkMode(cfg.Mode, client.Mode(), option.SelectDB); err != nil {
		client.Close()

		return nil, err
	}

	c := &Client{client: client, option: option, prefix: cfg.KeyPrefix}
	if err := c.Ping(ctx); err != nil {
		client.Close()

		return nil, err
	}

	return c, nil
}

func clientOption(cfg Config) (rueidis.ClientOption, error) {
	option, err := rueidis.ParseURL(cfg.URL)
	if err != nil {
		return option, fmt.Errorf("parse valkey url: %w", err)
	}

	option.InitAddress = append(option.InitAddress, cfg.Addrs...)

	switch cfg.Mode {
	case ModeAuto, ModeCluster:
	case ModeStandalone:
		option.ForceSingleClient = true
	case ModeSentinel:
		if cfg.Sentinel.Master == "" {
			return option, ErrSentinelMaster
		}

		option.Sentinel.MasterSet = cfg.Sentinel.Master
	default:
		return option, fmt.Errorf("%w %q", ErrInvalidMode, cfg.Mode)
	}

	if cfg.Mode == ModeCluster && option.SelectDB != 0 {
		return option, ErrClusterDatabase
	}

	return option, nil
}

func checkMode(want Mode, got rueidis.ClientMode, database int) error {
	if want == ModeCluster && got != rueidis.ClientModeCluster {
		return ErrNotCluster
	}

	if got == rueidis.ClientModeCluster && database != 0 {
		return ErrClusterDatabase
	}

	return nil
}

/*
Close closes every connection.
*/
func (c *Client) Close() {
	c.client.Close()
}

/*
Ping checks that the server answers.
*/
func (c *Client) Ping(ctx context.Context) error {
	if err := c.client.Do(ctx, c.client.B().Ping().Build()).Error(); err != nil {
		return fmt.Errorf("ping valkey: %w", err)
	}

	return nil
}

/*
Key builds the key for a keyspace, a tag and optional parts.

	c.Key("cache", "user:42")             // prefix + "cache:{user:42}"
	c.Key("rl", "login:ip", "1717000000") // prefix + "rl:{login:ip}:1717000000"

Valkey cluster hashes only the braced tag, so keys with the same keyspace
and tag always land in the same slot.
*/
func (c *Client) Key(keyspace, tag string, parts ...string) string {
	key := c.Prefix(keyspace) + "{" + tag + "}"
	if len(parts) == 0 {
		return key
	}

	return key + ":" + strings.Join(parts, ":")
}

/*
Prefix returns the common start of every key in a keyspace.
*/
func (c *Client) Prefix(keyspace string) string {
	return c.prefix + keyspace + ":"
}

/*
Rueidis returns the underlying client, for framework services that run
their own commands. App code reaches Valkey only through those services.
*/
func (c *Client) Rueidis() rueidis.Client {
	return c.client
}

/*
Option returns the options the client was built with, for framework
services that need a dedicated client of their own, such as locks and
cache-aside reads that rely on server-assisted client-side caching.
*/
func (c *Client) Option() rueidis.ClientOption {
	return c.option
}
