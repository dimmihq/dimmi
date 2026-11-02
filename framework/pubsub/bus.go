/*
|--------------------------------------------------------------------------
| Internal Bus
|--------------------------------------------------------------------------
|
| Processes tell each other that something changed: a cached value is
| stale, a setting moved, a job should stop. The bus is fire and forget.
| A notice reaches the processes listening at that moment and nobody else,
| so whatever a notice protects must also heal on its own, for example a
| local cache entry that expires.
|
| Topics live in their own keyspace under the client's prefix, so several
| applications can share one server. On a cluster the bus uses sharded
| channels, which keep each topic's traffic on the node that owns it
| instead of copying every notice to every node.
|
*/

package pubsub

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/redis/rueidis"

	"github.com/dimmihq/dimmi/framework/redis"
)

/*
Options configure a bus. The keyspace keeps its topics apart from every
other service on the same server.
*/
type Options struct {
	Keyspace string `default:"ps" koanf:"keyspace"`
}

/*
Handler receives the payload of each notice on a topic.

Handlers run one at a time per subscription and must return quickly: a
slow handler holds back delivery on the connection it shares. Hand long
work to a goroutine or a job.
*/
type Handler func(payload []byte)

/*
Unsubscribe stops a subscription and waits until its handler has
returned for the last time, so a handler must never call its own.
Calling it again does nothing.
*/
type Unsubscribe func()

/*
Bus publishes and receives notices on the shared Valkey client.

Subscriptions share the client's connection. A broken connection is
restored and every topic subscribed again by rueidis; notices published
while it was down are lost.
*/
type Bus struct {
	client   *redis.Client
	keyspace string
	sharded  bool

	mu        sync.Mutex
	listeners map[string]int
}

/*
New returns the bus for a keyspace on the given client.
*/
func New(client *redis.Client, opts Options) *Bus {
	return &Bus{
		client:    client,
		keyspace:  opts.Keyspace,
		sharded:   client.Rueidis().Mode() == rueidis.ClientModeCluster,
		listeners: map[string]int{},
	}
}

/*
Publish sends the payload to every process subscribed to the topic.
*/
func (b *Bus) Publish(ctx context.Context, topic string, payload []byte) error {
	rdb := b.client.Rueidis()

	if err := rdb.Do(ctx, b.publishCommand(rdb.B(), b.channel(topic), payload)).Error(); err != nil {
		return fmt.Errorf("publish %q: %w", topic, err)
	}

	return nil
}

/*
Subscribe calls fn with every notice published to the topic until the
returned Unsubscribe is called or the context ends.

It returns once the server has confirmed the subscription, so a notice
published after Subscribe returns is delivered. While the server cannot
be reached it keeps trying until the context ends.
*/
func (b *Bus) Subscribe(ctx context.Context, topic string, fn Handler) (Unsubscribe, error) {
	channel := b.channel(topic)
	listening, stop := context.WithCancel(ctx)
	confirmed, finished := make(chan struct{}), make(chan struct{})
	failed := make(chan error, 1)

	b.join(channel)

	go func() {
		defer close(finished)
		failed <- b.listen(listening, channel, fn, confirmed)
	}()

	select {
	case <-confirmed:
		return sync.OnceFunc(func() { stop(); <-finished }), nil
	case err := <-failed:
		stop()

		return nil, fmt.Errorf("subscribe %q: %w", topic, err)
	}
}

/*
listen receives notices until the context ends or the client closes.

The server ends a subscription on its own when a cluster slot moves, or
when another subscriber in this process left the topic at the same
moment; listen then subscribes again. Leaving is counted exactly once,
either by the receive hook that unsubscribes on the server or here.
*/
func (b *Bus) listen(ctx context.Context, channel string, fn Handler, confirmed chan<- struct{}) error {
	rdb := b.client.Rueidis()
	left := false
	hooked := rueidis.WithOnSubscriptionHook(ctx, confirmOnce(confirmed))
	hooked = rueidis.WithOnReceiveReturnHook(hooked, b.leaveOnReturn(ctx, channel, &left))
	deliver := func(msg rueidis.PubSubMessage) { fn([]byte(msg.Message)) }

	var err error
	for err == nil {
		err = rdb.Receive(hooked, b.subscribeCommand(rdb.B(), channel), deliver)
	}

	if !left {
		b.leave(channel)
	}

	return err
}

/*
leaveOnReturn runs on the subscribed connection each time a receive
returns. When the subscription has ended for good and no other
subscriber in this process listens to the channel, it unsubscribes on
the server, so the connection stops receiving notices nobody reads.
*/
func (b *Bus) leaveOnReturn(ctx context.Context, channel string, left *bool) func(error, rueidis.CommandClient) error {
	return func(err error, conn rueidis.CommandClient) error {
		if ctx.Err() == nil || *left {
			return err
		}

		*left = true
		if !b.leave(channel) {
			return err
		}

		return errors.Join(err, conn.Do(context.WithoutCancel(ctx), b.unsubscribeCommand(conn.B(), channel)).Error())
	}
}

func confirmOnce(confirmed chan<- struct{}) func(rueidis.PubSubSubscription) {
	var once sync.Once

	return func(s rueidis.PubSubSubscription) {
		if s.Kind == "subscribe" || s.Kind == "ssubscribe" {
			once.Do(func() { close(confirmed) })
		}
	}
}

func (b *Bus) join(channel string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.listeners[channel]++
}

/*
leave counts one subscriber out of the channel and reports whether it
was the last one in this process.
*/
func (b *Bus) leave(channel string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.listeners[channel]--
	if b.listeners[channel] > 0 {
		return false
	}

	delete(b.listeners, channel)

	return true
}

func (b *Bus) publishCommand(builder rueidis.Builder, channel string, payload []byte) rueidis.Completed {
	message := rueidis.BinaryString(payload)
	if b.sharded {
		return builder.Spublish().Channel(channel).Message(message).Build()
	}

	return builder.Publish().Channel(channel).Message(message).Build()
}

func (b *Bus) subscribeCommand(builder rueidis.Builder, channel string) rueidis.Completed {
	if b.sharded {
		return builder.Ssubscribe().Channel(channel).Build()
	}

	return builder.Subscribe().Channel(channel).Build()
}

func (b *Bus) unsubscribeCommand(builder rueidis.Builder, channel string) rueidis.Completed {
	if b.sharded {
		return builder.Sunsubscribe().Channel(channel).Build()
	}

	return builder.Unsubscribe().Channel(channel).Build()
}

/*
channel names the topic with Key, whose hash tag decides the cluster
node that carries a sharded channel.
*/
func (b *Bus) channel(topic string) string {
	return b.client.Key(b.keyspace, topic)
}
