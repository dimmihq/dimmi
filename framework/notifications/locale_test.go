package notifications_test

import (
	"context"
	"slices"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/i18n"
	"github.com/dimmihq/dimmi/framework/notifications"
	"github.com/dimmihq/dimmi/framework/queue"
)

/*
member is a recipient with a locale of its own, which may be empty.
*/
type member struct {
	ID     string
	Locale string
}

func (m member) NotifiableType() string { return "member" }

func (m member) NotifiableID() string { return m.ID }

func (m member) RouteFor(string) any { return m.ID }

/*
localized gives a member's locale through the Localized interface.
*/
type localized struct {
	member
}

func (l localized) Locale() string { return l.member.Locale }

/*
invite renders its text in the locale of the delivery.
*/
type invite struct {
	Name string `json:"name"`
}

func (invite) Type() string { return "invite" }

func (invite) Via(notifications.Notifiable) []string { return []string{"feed"} }

/*
rendering is what a channel rendered for one delivery.
*/
type rendering struct {
	To, Locale, Text string
}

/*
renderer is a channel that renders the invite with i18n.T and records
it.
*/
type renderer struct {
	mu   sync.Mutex
	seen []rendering
}

func (r *renderer) Send(ctx context.Context, d notifications.Delivery, n notifications.Notification) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	inv, _ := n.(invite)
	text := i18n.T(ctx, "notes.invite", i18n.Args{"name": inv.Name})
	r.seen = append(r.seen, rendering{To: d.Notifiable.NotifiableID(), Locale: d.Locale, Text: text})

	return nil
}

func (r *renderer) to(id string) rendering {
	r.mu.Lock()
	defer r.mu.Unlock()

	index := slices.IndexFunc(r.seen, func(seen rendering) bool { return seen.To == id })
	if index < 0 {
		return rendering{}
	}

	return r.seen[index]
}

/*
drain works the queue until the renderer saw the number of deliveries.
A job pushed a moment ago may not be due yet by the database's clock,
so draining once is not always enough.
*/
func (r *renderer) drain(t *testing.T, q *queue.Queue, deliveries int) {
	t.Helper()

	require.Eventually(t, func() bool {
		if err := q.Drain(t.Context()); err != nil {
			return false
		}

		r.mu.Lock()
		defer r.mu.Unlock()

		return len(r.seen) == deliveries
	}, 5*time.Second, 20*time.Millisecond)
}

/*
members holds the recipients the resolver loads, which a test may
change between queueing and delivery.
*/
type members struct {
	mu  sync.Mutex
	all map[string]member
}

func (m *members) set(who member) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.all[who.ID] = who
}

func (m *members) load(_ context.Context, id string) (notifications.Notifiable, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	who, ok := m.all[id]
	if !ok {
		return nil, notifications.ErrNotifiableGone
	}

	return localized{who}, nil
}

/*
newLocalized returns a manager with a rendering feed channel and the
English and French text of the invite.
*/
func newLocalized(t *testing.T, q *queue.Queue) (*notifications.Manager, *renderer, *members) {
	t.Helper()

	translator, err := i18n.New(i18n.Config{Locale: "en"}, fstest.MapFS{
		"en/notes.json": {Data: []byte(`{"invite": "{{.name}} invited you."}`)},
		"fr/notes.json": {Data: []byte(`{"invite": "{{.name}} vous a invité."}`)},
	})
	require.NoError(t, err)

	cfg := notifications.Config{
		Queue: "default", Channels: map[string]config.Section{"feed": {"driver": "render"}}, Translator: translator,
	}
	m, err := notifications.New(cfg, q)
	require.NoError(t, err)

	rec := &renderer{}
	m.Extend("render", func(string, config.Section) (notifications.Channel, error) { return rec, nil })
	notifications.Register[invite](m)

	people := &members{all: map[string]member{}}
	m.Resolve("member", people.load)

	return m, rec, people
}

func TestLocalizedDelivery(t *testing.T) {
	t.Parallel()

	t.Run("renders queued deliveries in each recipient's locale", func(t *testing.T) {
		t.Parallel()
		q, db := setup(t)
		m, rec, people := newLocalized(t, q)
		zoe, ana := member{ID: "zoe", Locale: "fr"}, member{ID: "ana"}
		people.set(zoe)
		people.set(ana)
		sender := i18n.WithLocale(t.Context(), "en")

		err := db.Transaction(sender, func(ctx context.Context, tx *database.Tx) error {
			return m.SendTx(ctx, tx, invite{Name: "Bo"}, localized{zoe}, localized{ana})
		})
		require.NoError(t, err)
		rec.drain(t, q, 2)

		assert.Equal(t, rendering{To: "zoe", Locale: "fr", Text: "Bo vous a invité."}, rec.to("zoe"))
		assert.Equal(t, rendering{To: "ana", Locale: "en", Text: "Bo invited you."}, rec.to("ana"))
	})

	t.Run("follows a locale the recipient picked after queueing", func(t *testing.T) {
		t.Parallel()
		q, _ := setup(t)
		m, rec, people := newLocalized(t, q)
		people.set(member{ID: "zoe"})

		require.NoError(t, m.SendTx(t.Context(), nil, invite{Name: "Bo"}, member{ID: "zoe"}))
		people.set(member{ID: "zoe", Locale: "fr"})
		rec.drain(t, q, 1)

		assert.Equal(t, rendering{To: "zoe", Locale: "fr", Text: "Bo vous a invité."}, rec.to("zoe"))
	})

	t.Run("renders deliveries sent now in the recipient's locale", func(t *testing.T) {
		t.Parallel()
		m, rec, _ := newLocalized(t, nil)
		sender := i18n.WithLocale(t.Context(), "en")

		zoe := localized{member{ID: "zoe", Locale: "fr"}}
		require.NoError(t, m.Send(sender, invite{Name: "Bo"}, zoe, member{ID: "ana"}))

		assert.Equal(t, "Bo vous a invité.", rec.to("zoe").Text)
		assert.Equal(t, rendering{To: "ana", Locale: "en", Text: "Bo invited you."}, rec.to("ana"))
	})
}
