//go:build emailmatrix

package emailmatrix

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/filesystem/drivers/local"
	"github.com/dimmihq/dimmi/framework/mail"
)

/*
world is an in-memory stand-in for the mail system and the application, so the
scenarios can be proven without sending anything.
*/
type world struct {
	mu        sync.Mutex
	forwarded []mail.Message
	replies   []string
	spamHost  string
	keepQuote bool
	authFail  bool
}

func (w *world) markerConversation(subject string) string {
	fields := strings.Fields(subject)

	return fields[len(fields)-1]
}

func (w *world) Inbound(_ context.Context, _, marker string) (inboundMail, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	for _, message := range w.forwarded {
		if strings.Contains(message.Subject, marker) {
			auth := "pass"
			if w.authFail {
				auth = "fail"
			}

			return inboundMail{
				Found: true, ConversationID: w.markerConversation(message.Subject), AuthResult: auth,
			}, nil
		}
	}

	return inboundMail{}, nil
}

func (w *world) Reply(_ context.Context, _, body string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.replies = append(w.replies, body)

	return nil
}

func (w *world) Timeline(_ context.Context, conversationID string) (timeline, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	current := timeline{ContactID: "contact"}

	for _, message := range w.forwarded {
		if w.markerConversation(message.Subject) != conversationID {
			continue
		}

		body := message.Text
		if !w.keepQuote {
			body, _, _ = strings.Cut(body, "\n\n>")
		}

		current.Bodies = append(current.Bodies, body)
	}

	return current, nil
}

type worldBox struct {
	world *world
	host  string
}

func (b worldBox) Send(_ context.Context, msg mail.Message) error {
	b.world.mu.Lock()
	defer b.world.mu.Unlock()

	b.world.forwarded = append(b.world.forwarded, msg)

	return nil
}

func (b worldBox) Find(_ context.Context, marker string) (received, error) {
	b.world.mu.Lock()
	defer b.world.mu.Unlock()

	for _, reply := range b.world.replies {
		if !strings.Contains(reply, marker) {
			continue
		}

		folder := folderInbox
		if b.host == b.world.spamHost {
			folder = "spam"
		}

		return received{
			Found:  true,
			Folder: folder,
			Headers: map[string][]string{
				"Authentication-Results": {"mx; spf=pass; dkim=pass; dmarc=pass"},
				"In-Reply-To":            {"<a@example.test>"},
				"Message-Id":             {"<b@example.test>"},
			},
			Text: reply,
		}, nil
	}

	return received{}, nil
}

func newHarness(w *world) *harness {
	boxes := make([]mailbox, 0, len(mailboxHosts()))
	for _, host := range mailboxHosts() {
		box := worldBox{world: w, host: host.Name}
		boxes = append(boxes, mailbox{Host: host.Name, Address: host.Name + "@example.test", Out: box, In: box})
	}

	return &harness{
		Clock: clock.System(), Product: w, Mailboxes: boxes, Forwarding: "support@example.test", AddressID: "address",
		Wait: 20 * time.Millisecond, AnswerBar: time.Minute, Poll: time.Millisecond,
	}
}

func completeEnviron() []string {
	environ := []string{
		"EMAIL_MATRIX_ROOT=/tmp/root",
		"EMAIL_MATRIX_BASE_URL=https://staging.example.test",
		"EMAIL_MATRIX_API_TOKEN=secret-api-token",
		"EMAIL_MATRIX_FORWARDING=support@example.test",
		"EMAIL_MATRIX_ADDRESS_ID=address",
	}

	for _, host := range mailboxHosts() {
		prefix := "EMAIL_MATRIX_MAILBOXES__" + strings.ToUpper(host.Name) + "__"
		environ = append(environ,
			prefix+"ADDRESS="+host.Name+"@example.test",
			prefix+"TOKEN=secret-token",
			prefix+"SMTP_HOST=smtp.example.test",
			prefix+"SMTP_PASSWORD=secret-password",
		)
	}

	return environ
}

type storeFunc func(ctx context.Context, id string) ([]byte, error)

func (f storeFunc) Read(ctx context.Context, id string) ([]byte, error) {
	return f(ctx, id)
}

func TestSettings(t *testing.T) {
	t.Parallel()

	noStore := storeFunc(func(context.Context, string) ([]byte, error) {
		return nil, errors.New("the store must not be read")
	})

	t.Run("it refuses to run with no secrets and names what is missing", func(t *testing.T) {
		t.Parallel()

		_, _, err := loadSettings(t.Context(), []string{}, noStore)

		var missing *missingSecretsError
		require.ErrorAs(t, err, &missing)
		assert.Contains(t, missing.Names, "api_token")
		assert.Contains(t, missing.Names, "mailboxes.gmail.address")
		assert.Contains(t, missing.Names, "mailboxes.m365.token")
	})

	t.Run("it starts when every secret is present", func(t *testing.T) {
		t.Parallel()

		conf, run, err := loadSettings(t.Context(), completeEnviron(), noStore)

		require.NoError(t, err)
		assert.Equal(t, "secret-api-token", conf.APIToken)
		assert.Len(t, conf.Mailboxes, 6)
		assert.Empty(t, run.Adapter)
	})

	t.Run("a refusal never prints a value", func(t *testing.T) {
		t.Parallel()

		environ := completeEnviron()[1:]

		_, _, err := loadSettings(t.Context(), environ, noStore)

		require.Error(t, err)
		assert.NotContains(t, err.Error(), "secret-")
	})

	t.Run("secrets fill in from the store when it is named", func(t *testing.T) {
		t.Parallel()

		document := `{"base_url":"https://staging.example.test","api_token":"t","forwarding":"f@example.test",` +
			`"address_id":"a","mailboxes":{` +
			`"gmail":{"address":"a","smtp_host":"h","smtp_password":"p"},` +
			`"workspace":{"address":"a","token":"t","smtp_host":"h","smtp_password":"p"},` +
			`"outlook":{"address":"a","token":"t","smtp_host":"h","smtp_password":"p"},` +
			`"m365":{"address":"a","token":"t"},` +
			`"yahoo":{"address":"a","smtp_host":"h","smtp_password":"p"},` +
			`"icloud":{"address":"a","smtp_host":"h","smtp_password":"p"}}}`
		asked := ""
		store := storeFunc(func(_ context.Context, id string) ([]byte, error) {
			asked = id

			return []byte(document), nil
		})

		conf, _, err := loadSettings(t.Context(), []string{
			"EMAIL_MATRIX_ROOT=/tmp/root", "EMAIL_MATRIX_SECRETS_ID=dimmi/email-matrix/all",
		}, store)

		require.NoError(t, err)
		assert.Equal(t, "dimmi/email-matrix/all", asked)
		assert.Equal(t, "t", conf.APIToken)
	})

	t.Run("a broken secret document is refused without echoing it", func(t *testing.T) {
		t.Parallel()

		store := storeFunc(func(context.Context, string) ([]byte, error) {
			return []byte("not json secret-value"), nil
		})

		_, _, err := loadSettings(t.Context(), []string{"EMAIL_MATRIX_SECRETS_ID=x"}, store)

		require.Error(t, err)
		assert.NotContains(t, err.Error(), "secret-value")
	})

	t.Run("an adapter run also needs that adapter's target", func(t *testing.T) {
		t.Parallel()

		environ := append(completeEnviron(), "EMAIL_MATRIX_ARGS=-adapter postmark")

		_, _, err := loadSettings(t.Context(), environ, noStore)

		var missing *missingSecretsError
		require.ErrorAs(t, err, &missing)
		assert.Contains(t, missing.Names, "adapters.postmark.domain")
	})
}

func TestParseFlags(t *testing.T) {
	t.Parallel()

	t.Run("it reads both flags in the forms a person types", func(t *testing.T) {
		t.Parallel()

		for _, args := range []string{"-adapter postmark -record", "--adapter=postmark --record"} {
			parsed, err := parseFlags(args)

			require.NoError(t, err)
			assert.Equal(t, flags{Adapter: "postmark", Record: true}, parsed)
		}
	})

	t.Run("it refuses what it does not know", func(t *testing.T) {
		t.Parallel()

		for _, args := range []string{"-nope", "-adapter", "-adapter nope", "-record"} {
			_, err := parseFlags(args)

			require.ErrorIs(t, err, errUnknownFlag, args)
		}
	})
}

func TestMatrix(t *testing.T) {
	t.Parallel()

	statuses := func(r report, host string) map[string]string {
		found := map[string]string{}

		for _, entry := range r.Results {
			if entry.Host == host {
				found[entry.Scenario] = entry.Status
			}
		}

		return found
	}

	t.Run("every host passes every scenario when mail behaves", func(t *testing.T) {
		t.Parallel()

		finished := newHarness(&world{}).runMatrix(t.Context())

		assert.Len(t, finished.Results, 24)
		assert.True(t, finished.passed(), finished.Results)
	})

	t.Run("a spam folder fails that host and blocks what depends on it", func(t *testing.T) {
		t.Parallel()

		finished := newHarness(&world{spamHost: "yahoo"}).runMatrix(t.Context())
		yahoo := statuses(finished, "yahoo")

		assert.False(t, finished.passed())
		assert.Equal(t, statusPass, yahoo[scenarioNewMail])
		assert.Equal(t, statusFail, yahoo[scenarioReply])
		assert.Equal(t, statusBlocked, yahoo[scenarioAnswer])
		assert.Equal(t, statusPass, statuses(finished, "gmail")[scenarioReply])
		assert.Equal(t, []string{"yahoo"}, finished.spamHosts())
	})

	t.Run("an unverified sender fails the first scenario", func(t *testing.T) {
		t.Parallel()

		finished := newHarness(&world{authFail: true}).runMatrix(t.Context())

		assert.Equal(t, statusFail, statuses(finished, "gmail")[scenarioNewMail])
	})

	t.Run("a quote left in the conversation fails the answer", func(t *testing.T) {
		t.Parallel()

		finished := newHarness(&world{keepQuote: true}).runMatrix(t.Context())

		assert.Equal(t, statusFail, statuses(finished, "gmail")[scenarioAnswer])
	})

	t.Run("a way of reaching a host that is not built blocks and never passes", func(t *testing.T) {
		t.Parallel()

		engine := newHarness(&world{})
		engine.Mailboxes[0].In = unavailableReader{via: "imap"}

		finished := engine.runMatrix(t.Context())

		assert.Equal(t, statusBlocked, statuses(finished, "gmail")[scenarioReply])
		assert.False(t, finished.passed())
	})

	t.Run("real credentials open real senders but no readers yet", func(t *testing.T) {
		t.Parallel()

		secrets := map[string]mailboxSecret{}
		for _, host := range mailboxHosts() {
			secrets[host.Name] = mailboxSecret{Address: "a@example.test", SMTPHost: "smtp.example.test", Token: "t"}
		}

		boxes, err := openMailboxes(clock.System(), secrets)

		require.NoError(t, err)
		require.Len(t, boxes, 6)

		_, err = boxes[0].In.Find(t.Context(), "marker")
		require.ErrorIs(t, err, errBlocked)
		assert.IsType(t, smtpSender{}, boxes[0].Out)
	})
}

func TestAdapters(t *testing.T) {
	t.Parallel()

	run := func(w *world) adapterRun {
		box := worldBox{world: w, host: "gmail"}

		return adapterRun{
			Adapter:  adapterByName("postmark"),
			Target:   adapterTarget{Domain: "pm.example.test", AddressID: "pm"},
			From:     mailbox{Host: "gmail", Address: "gmail@example.test", Out: box, In: box},
			Payloads: unavailablePayloads{},
		}
	}

	t.Run("a provider passes when its mail arrives with a verdict", func(t *testing.T) {
		t.Parallel()

		w := &world{}

		finished := newHarness(w).runAdapter(t.Context(), run(w))

		assert.True(t, finished.passed(), finished.Results)
		assert.Equal(t, "x@pm.example.test", w.forwarded[0].To[0].Email)
	})

	t.Run("recording is blocked until payloads can be read", func(t *testing.T) {
		t.Parallel()

		w := &world{}
		selected := run(w)

		disk, err := local.New(local.Options{Root: t.TempDir()})
		require.NoError(t, err)

		selected.Disk = disk

		finished := newHarness(w).runAdapter(t.Context(), selected)

		require.Len(t, finished.Results, 2)
		assert.Equal(t, statusBlocked, finished.Results[1].Status)
	})

	t.Run("a connected mailbox adapter blocks until its driver exists", func(t *testing.T) {
		t.Parallel()

		w := &world{}
		selected := run(w)
		selected.Adapter = adapterByName("imap")

		finished := newHarness(w).runAdapter(t.Context(), selected)

		assert.Equal(t, statusBlocked, finished.Results[0].Status)
	})

	t.Run("sanitizing removes addresses, locations and secrets but keeps the shape", func(t *testing.T) {
		t.Parallel()

		payload := `{"From":"real.person@company.io","Signature":"abc123",` +
			`"api_key":"k","Ip":"203.0.113.9","Tag":"keep"}`

		cleaned := string(sanitize([]byte(payload)))

		assert.JSONEq(t,
			`{"From":"person@example.test","Signature":"redacted","api_key":"redacted","Ip":"192.0.2.1","Tag":"keep"}`,
			cleaned,
		)
	})
}

func TestReport(t *testing.T) {
	t.Parallel()

	spam := report{Results: []result{{Host: "yahoo", Scenario: "s", Status: statusFail, SpamFolder: true}}}
	clean := report{Results: []result{{Host: "yahoo", Scenario: "s", Status: statusPass}}}

	t.Run("spam twice in a row holds the release", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, []string{"yahoo"}, spam.spamTwice(spam))
		assert.Empty(t, spam.spamTwice(clean))
		require.ErrorContains(t, verdict(spam, spam), "release is held")
	})

	t.Run("an empty run does not pass", func(t *testing.T) {
		t.Parallel()

		assert.False(t, report{}.passed())
		require.NoError(t, verdict(clean, report{}))
	})

	t.Run("the artifacts are written as json and a table and read back", func(t *testing.T) {
		t.Parallel()

		disk, err := local.New(local.Options{Root: t.TempDir()})
		require.NoError(t, err)

		var printed strings.Builder

		require.Error(t, conclude(t.Context(), disk, spam, &printed))
		assert.Contains(t, printed.String(), "yahoo")
		assert.Contains(t, printed.String(), "spam folder: 1 host(s)")

		previous, err := loadPrevious(t.Context(), disk)
		require.NoError(t, err)
		assert.Equal(t, spam, previous)

		require.ErrorContains(t, conclude(t.Context(), disk, spam, &printed), "release is held")
	})
}
