/*
|--------------------------------------------------------------------------
| Queued Mail
|--------------------------------------------------------------------------
|
| Mail an action sends is queued in the action's own transaction, so it
| leaves only when the work it reports was committed, and a provider
| outage delays it instead of failing the action. The job stores the
| mailable's data under the name it was registered with, never its Go
| type, and the worker renders it when it runs, in the locale the job
| recorded: the one the mail named, or else the locale of the context it
| was queued from.
|
|   mail.Register[SignInMail](a.Mail, "sign_in")
|   err := a.Mail.To(addr).QueueTx(ctx, tx, SignInMail{URL: url})
|
| Retries follow the error kind: a temporary failure retries with a
| growing delay, a rate limit waits as long as the provider asked
| without using up an attempt, and a permanent failure gives the job up.
|
*/

package mail

import (
	"cmp"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/i18n"
	"github.com/dimmihq/dimmi/framework/queue"
)

/*
Errors from queueing mail: a mailable type no one registered, or a
manager that was given no queue.
*/
var (
	ErrUnregisteredMailable = errors.New("mail: mailable not registered")
	ErrNoQueue              = errors.New("mail: no queue configured")
)

/*
rateLimitSpread bounds the wait of a rate limited job when the provider
did not say how long: a random wait between one second and a minute, so
jobs held back together do not all retry in the same instant.
*/
const rateLimitSpread = time.Minute

/*
rateLimitWait picks the wait within the spread.
*/
func rateLimitWait() time.Duration {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(rateLimitSpread-time.Second)))
	if err != nil {
		return rateLimitSpread / 2
	}

	return time.Second + time.Duration(n.Int64())
}

/*
SendArgs is the job that sends one queued mailable: the mailer to send
on, the mailable's registered name and data, the locale it renders in
and its recipients.
*/
type SendArgs struct {
	Mailer   string          `json:"mailer"`
	Mailable string          `json:"mailable"`
	Locale   string          `json:"locale,omitempty"`
	Payload  json.RawMessage `json:"payload"`
	To       []Address       `json:"to"`
	Cc       []Address       `json:"cc,omitempty"`
	Bcc      []Address       `json:"bcc,omitempty"`
}

/*
Kind names the job type.
*/
func (SendArgs) Kind() string {
	return "send_mail"
}

/*
registration ties a mailable type to the name its jobs store.
*/
type registration struct {
	name    string
	matches func(mailable Mailable) bool
	decode  func(payload json.RawMessage) (Mailable, error)
}

/*
Register lets mailables of type M be queued under the name. The name is
stored in every job, so it must stay stable while jobs may wait; M must
encode to JSON and back without losing what it renders.
*/
func Register[M Mailable](m *Manager, name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.mailables = append(m.mailables, registration{
		name: name,
		matches: func(mailable Mailable) bool {
			_, ok := mailable.(M)

			return ok
		},
		decode: func(payload json.RawMessage) (Mailable, error) {
			var mailable M
			err := json.Unmarshal(payload, &mailable)

			return mailable, err
		},
	})
}

/*
UseQueue queues mail on the queue and registers the worker that sends
it. Call it before the queue starts.
*/
func (m *Manager) UseQueue(q *queue.Queue) error {
	m.mu.Lock()
	m.queue = q
	m.mu.Unlock()

	return queue.Register(q, sendWorker{manager: m})
}

/*
QueueTx queues the mailable in the transaction, so it is sent only if
the transaction commits.
*/
func (p *PendingMail) QueueTx(ctx context.Context, tx *database.Tx, mailable Mailable) error {
	q, args, err := p.job(ctx, mailable)
	if err != nil {
		return err
	}

	return q.PushTx(ctx, tx, args)
}

/*
Queue queues the mailable outside any transaction.
*/
func (p *PendingMail) Queue(ctx context.Context, mailable Mailable) error {
	q, args, err := p.job(ctx, mailable)
	if err != nil {
		return err
	}

	return q.Push(ctx, args)
}

func (p *PendingMail) job(ctx context.Context, mailable Mailable) (*queue.Queue, SendArgs, error) {
	m := p.manager

	m.mu.Lock()
	q, mailables := m.queue, m.mailables
	m.mu.Unlock()

	if q == nil {
		return nil, SendArgs{}, ErrNoQueue
	}

	for _, r := range mailables {
		if !r.matches(mailable) {
			continue
		}

		payload, err := json.Marshal(mailable)
		if err != nil {
			return nil, SendArgs{}, fmt.Errorf("encode mailable %s: %w", r.name, err)
		}

		return q, SendArgs{
			Mailer: p.mailerName(), Mailable: r.name, Locale: cmp.Or(p.locale, i18n.Locale(ctx)),
			Payload: payload, To: p.to, Cc: p.cc, Bcc: p.bcc,
		}, nil
	}

	return nil, SendArgs{}, fmt.Errorf("%w: %T", ErrUnregisteredMailable, mailable)
}

/*
decode rebuilds the mailable a job stores.
*/
func (m *Manager) decode(args SendArgs) (Mailable, error) {
	m.mu.Lock()
	mailables := m.mailables
	m.mu.Unlock()

	for _, r := range mailables {
		if r.name != args.Mailable {
			continue
		}

		mailable, err := r.decode(args.Payload)
		if err != nil {
			return nil, fmt.Errorf("decode mailable %s: %w", args.Mailable, err)
		}

		return mailable, nil
	}

	return nil, fmt.Errorf("%w: %s", ErrUnregisteredMailable, args.Mailable)
}

/*
sendWorker sends queued mailables.
*/
type sendWorker struct {
	manager *Manager
}

/*
Handle renders the mailable in the locale the job recorded and sends
it, telling the queue what to do when sending fails. A job whose
mailable cannot be rebuilt never will be, so it is given up.
*/
func (w sendWorker) Handle(ctx context.Context, job *queue.Job[SendArgs]) error {
	mailable, err := w.manager.decode(job.Args)
	if err != nil {
		return queue.Cancel(err)
	}

	pending := &PendingMail{
		manager: w.manager, locale: job.Args.Locale, to: job.Args.To, cc: job.Args.Cc, bcc: job.Args.Bcc,
	}
	_, err = pending.sendOn(ctx, job.Args.Mailer, mailable)

	return retry(err)
}

func retry(err error) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, ErrPermanent) {
		return queue.Cancel(err)
	}

	if limited, ok := errors.AsType[*RateLimitedError](err); ok {
		return queue.Postpone(cmp.Or(limited.RetryAfter, rateLimitWait()))
	}

	return err
}
