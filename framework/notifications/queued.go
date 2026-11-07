/*
|--------------------------------------------------------------------------
| Queued Delivery
|--------------------------------------------------------------------------
|
| A notification about something saved in a transaction must go out
| only if the transaction commits. Each delivery becomes a job pushed in
| that transaction: one per recipient and channel, so a channel that
| fails is retried on its own without repeating the ones that worked.
|
| The job stores the notification as JSON with its type, and the
| recipient by its type and id. When it runs, the notification is
| decoded into the registered type and the recipient is loaded again, so
| the delivery sees the recipient as it is then, not as it was. The job
| also records the locale the delivery was meant for; a recipient that
| has since picked another locale is served in the new one.
|
| A failure that can never succeed gives the job up; any other failure
| retries it.
|
*/

package notifications

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"
)

/*
deliverArgs is the job of one queued delivery.
*/
type deliverArgs struct {
	ID             string          `json:"id"`
	Channel        string          `json:"channel"`
	Locale         string          `json:"locale,omitempty"`
	Type           string          `json:"type"`
	Notification   json.RawMessage `json:"notification"`
	NotifiableType string          `json:"notifiable_type"`
	NotifiableID   string          `json:"notifiable_id"`
}

/*
Kind names the job type.
*/
func (deliverArgs) Kind() string { return "deliver_notification" }

/*
Register makes notifications of type N deliverable from the queue. N is
decoded from the JSON it was encoded to when queued, so the fields that
matter must be exported. Register every type before the queue starts.
*/
func Register[N Notification](m *Manager) {
	var zero N

	m.mu.Lock()
	defer m.mu.Unlock()

	m.types[zero.Type()] = func(data []byte) (Notification, error) {
		var n N
		if err := json.Unmarshal(data, &n); err != nil {
			return nil, fmt.Errorf("decode %s: %w", zero.Type(), err)
		}

		return n, nil
	}
}

/*
RegisterWith makes notifications of type N deliverable from the queue
like Register, decoding each queued one into the value build returns.
A notification that needs services when it is delivered, such as to
check at send time whether it still matters, gets them from build;
they are not encoded, so keep them out of JSON.
*/
func RegisterWith[N Notification](m *Manager, build func() N) {
	zero := build()

	m.mu.Lock()
	defer m.mu.Unlock()

	m.types[zero.Type()] = func(data []byte) (Notification, error) {
		n := build()
		if err := json.Unmarshal(data, n); err != nil {
			return nil, fmt.Errorf("decode %s: %w", zero.Type(), err)
		}

		return n, nil
	}
}

/*
SendTx queues the notification for every recipient, one job per channel
it picks, inside the transaction: nothing is delivered unless the
transaction commits. With a nil transaction the jobs are pushed on their
own. The notification type and every recipient type must be registered.
*/
func (m *Manager) SendTx(ctx context.Context, tx *database.Tx, n Notification, to ...Notifiable) error {
	if m.queue == nil {
		return fmt.Errorf("queue %s: %w", n.Type(), ErrNoQueue)
	}

	data, err := m.encode(n)
	if err != nil {
		return err
	}

	for _, notifiable := range to {
		if _, err := m.resolver(notifiable.NotifiableType()); err != nil {
			return fmt.Errorf("queue %s: %w", n.Type(), err)
		}

		id := ids.New().String()

		for _, channel := range n.Via(notifiable) {
			d := Delivery{ID: id, Channel: channel, Locale: localeOf(ctx, notifiable), Notifiable: notifiable}
			args, opts := m.job(d, n, data)
			if err := m.push(ctx, tx, args, opts); err != nil {
				return err
			}
		}
	}

	return nil
}

/*
encode stores the notification as JSON, refusing a type that the
worker could not decode again.
*/
func (m *Manager) encode(n Notification) ([]byte, error) {
	if _, err := m.decoder(n.Type()); err != nil {
		return nil, fmt.Errorf("queue %s: %w", n.Type(), err)
	}

	data, err := json.Marshal(n)
	if err != nil {
		return nil, fmt.Errorf("encode %s: %w", n.Type(), err)
	}

	return data, nil
}

/*
job builds the job of one delivery and its options: the configured
queue unless the notification picks others.
*/
func (m *Manager) job(d Delivery, n Notification, data []byte) (deliverArgs, []queue.Opt) {
	args := deliverArgs{
		ID:             d.ID,
		Channel:        d.Channel,
		Locale:         d.Locale,
		Type:           n.Type(),
		Notification:   data,
		NotifiableType: d.Notifiable.NotifiableType(),
		NotifiableID:   d.Notifiable.NotifiableID(),
	}

	opts := []queue.Opt{queue.OnQueue(m.queueName)}
	if queued, ok := n.(Queued); ok {
		opts = append(opts, queued.QueueOptions(d)...)
	}

	return args, opts
}

/*
push adds the job in the transaction, or on its own without one.
*/
func (m *Manager) push(ctx context.Context, tx *database.Tx, args deliverArgs, opts []queue.Opt) error {
	var err error
	if tx == nil {
		err = m.queue.Push(ctx, args, opts...)
	} else {
		err = m.queue.PushTx(ctx, tx, args, opts...)
	}

	if err != nil {
		return fmt.Errorf("queue %s via %s: %w", args.Type, args.Channel, err)
	}

	return nil
}

func (m *Manager) decoder(notificationType string) (decoder, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	decode, ok := m.types[notificationType]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnregistered, notificationType)
	}

	return decode, nil
}

func (m *Manager) resolver(notifiableType string) (Resolver, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	resolve, ok := m.resolvers[notifiableType]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrNoResolver, notifiableType)
	}

	return resolve, nil
}

/*
deliverWorker runs queued deliveries.
*/
type deliverWorker struct {
	manager *Manager
}

/*
Handle decodes the notification, loads the recipient and delivers in
the locale the recipient has now, or else the one the job recorded. A
recipient that is gone drops the delivery; a failure that can never
succeed gives the job up.
*/
func (w deliverWorker) Handle(ctx context.Context, job *queue.Job[deliverArgs]) error {
	args := job.Args

	n, to, err := w.load(ctx, args)
	if errors.Is(err, ErrNotifiableGone) {
		return nil
	}

	if err != nil {
		return err
	}

	d := Delivery{ID: args.ID, Channel: args.Channel, Locale: cmp.Or(ownLocale(to), args.Locale), Notifiable: to}

	err = w.manager.deliver(ctx, d, n)
	if errors.Is(err, ErrPermanent) || errors.Is(err, ErrUnsupported) || errors.Is(err, ErrInvalidRoute) {
		return queue.Cancel(err)
	}

	return err
}

/*
load rebuilds the notification and the recipient of a job. JSON that no
longer decodes gives the job up. A type or resolver missing from this
node is retried instead, since during a deploy another node may know it.
*/
func (w deliverWorker) load(ctx context.Context, args deliverArgs) (Notification, Notifiable, error) {
	decode, err := w.manager.decoder(args.Type)
	if err != nil {
		return nil, nil, err
	}

	n, err := decode(args.Notification)
	if err != nil {
		return nil, nil, queue.Cancel(err)
	}

	resolve, err := w.manager.resolver(args.NotifiableType)
	if err != nil {
		return nil, nil, err
	}

	to, err := resolve(ctx, args.NotifiableID)
	if err != nil {
		return nil, nil, fmt.Errorf("load %s %s: %w", args.NotifiableType, args.NotifiableID, err)
	}

	return n, to, nil
}
