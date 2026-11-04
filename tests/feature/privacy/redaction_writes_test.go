package privacy_test

import (
	"bytes"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

const card = "4242 4242 4242 4242"

/*
start opens a conversation a customer wrote, through the API channel,
and returns its path.
*/
func start(t *testing.T, d tests.Desk, subject, body string) string {
	t.Helper()

	res := d.Owner.PostJSON("/api/v1/conversations", testkit.JSON{
		"inbox_id": d.InboxID(), "author": "contact", "subject": subject, "body": body,
		"contact": testkit.JSON{"email": "ann@example.com", "name": "Ann"},
	}).AssertCreated()

	return "/api/v1/conversations/" + res.String("data.id")
}

func TestRedactionOnWrite(t *testing.T) {
	t.Parallel()

	t.Run("masks a card in what a customer writes and its subject, keeping only the count", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		redacted := tests.Record[appevents.MessageRedacted](d.TC)

		path := start(t, d, "Card "+card, "My card is "+card+", and again 5555555555554444")

		d.Owner.GetJSON(path).AssertOK().AssertJSONPath("data.subject", "Card •••• •••• •••• 4242")
		d.Owner.GetJSON(path+"/messages").AssertOK().
			AssertJSONPath("data.0.body", "My card is •••• •••• •••• 4242, and again •••• •••• •••• 4444").
			AssertJSONPath("data.0.redactions.0.rule", "card_numbers").
			AssertJSONPath("data.0.redactions.0.count", 2)
		require.Len(t, redacted.All(), 1)
		assert.Equal(t, []string{"card_numbers"}, redacted.All()[0].Rules)
		assert.Equal(t, 2, redacted.All()[0].Count)
		original := "My card is " + card + ", and again 5555555555554444"
		d.TC.Table("messages").AssertCount(0, testkit.Row{"body": original})
	})

	t.Run("applies the workspace's rules to replies, notes, follow-ups and a new subject", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		d.Owner.PostJSON(rules, testkit.JSON{"name": "Accounts", "pattern": `ACCT-\d+`, "replacement": "ACCT-•"}).
			AssertCreated()
		d.Owner.PatchJSON(rules, testkit.JSON{"us_ssn": true}).AssertOK()
		path := start(t, d, "Hello", "Hi")

		d.Owner.PostJSON(path+"/replies", testkit.JSON{"body": "Your ACCT-123 is fine"}).AssertCreated().
			AssertJSONPath("data.body", "Your ACCT-• is fine").
			AssertJSONPath("data.redactions.0.rule", d.Owner.GetJSON(rules).String("data.rules.0.id"))
		d.Owner.PostJSON(path+"/notes", testkit.JSON{"body": "SSN 123-45-6789"}).AssertCreated().
			AssertJSONPath("data.body", "SSN •••-••-••••")
		d.Owner.PostJSON(path+"/messages", testkit.JSON{"body": "ACCT-9 and 000-12-3456"}).AssertCreated()
		d.Owner.PatchJSON(path, testkit.JSON{"subject": "About ACCT-77"}).AssertOK().
			AssertJSONPath("data.subject", "About ACCT-•")

		d.Owner.GetJSON(path+"/messages?filter[type]=message").AssertOK().
			AssertJSONPath("data.2.body", "ACCT-• and 000-12-3456")
	})

	t.Run("leaves text alone when the rules are off and never rewrites what was stored", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		d.Owner.PatchJSON(rules, testkit.JSON{"card_numbers": false}).AssertOK()
		id := d.Owner.PostJSON(rules, testkit.JSON{"name": "Off", "pattern": "secret", "enabled": false}).
			AssertCreated().String("data.id")

		path := start(t, d, "Hi", "secret "+card)
		d.Owner.PatchJSON(rules, testkit.JSON{"card_numbers": true}).AssertOK()
		d.Owner.PatchJSON(rules+"/"+id, testkit.JSON{"enabled": true}).AssertOK()

		d.Owner.GetJSON(path+"/messages").AssertOK().
			AssertJSONPath("data.0.body", "secret "+card).
			AssertJSONMissing("data.0.redactions")
	})

	t.Run("tries text against the rules without storing it", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		d.Owner.PostJSON(rules, testkit.JSON{"name": "Pins", "pattern": `PIN \d{4}`}).AssertCreated()

		d.Owner.PostJSON("/api/v1/redaction-tests", testkit.JSON{"text": "PIN 1234 card " + card}).AssertOK().
			AssertJSONPath("data.object", "redaction_test").
			AssertJSONPath("data.text", "[redacted] card •••• •••• •••• 4242").
			AssertJSONPath("data.redactions.#", 2)
		d.Owner.PostJSON("/api/v1/redaction-tests", testkit.JSON{"text": "nothing here"}).AssertOK().
			AssertJSONPath("data.redactions.#", 0)
		d.TC.Table("messages").AssertCount(0)
	})
}

func TestRedactionLogs(t *testing.T) {
	t.Parallel()

	t.Run("never writes the original text to the logs", func(t *testing.T) {
		t.Parallel()
		logs := &syncBuffer{}
		tc := tests.New(t, tests.WithLogs(logs), tests.WithEnv("DIMMI_LOG__LEVEL=debug", "DIMMI_LOG__DRIVER=json"))
		inbox, err := factories.DefaultInbox(t.Context(), tc.App, tc.Workspace.Workspace)
		require.NoError(t, err)
		d := tests.Desk{TC: tc, Inbox: inbox, Owner: tc.In(tc.Owner)}
		path := start(t, d, "Card "+card, "Pay with "+card)

		d.Owner.PostJSON(path+"/replies", testkit.JSON{"body": "Got 4242424242424242"}).AssertCreated()
		d.Owner.PostJSON("/api/v1/redaction-tests", testkit.JSON{"text": "try " + card}).AssertOK()
		tc.FailWrites("messages", "INSERT")
		d.Owner.PostJSON(path+"/replies", testkit.JSON{"body": "Again 4242424242424242"}).AssertStatus(500)

		assert.Contains(t, logs.String(), "request failed")
		assert.NotContains(t, logs.String(), "4242 4242 4242 4242")
		assert.NotContains(t, logs.String(), "4242424242424242")
	})
}

/*
syncBuffer collects log lines written from any goroutine.
*/
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

/*
Write keeps the line.
*/
func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.Write(p)
}

/*
String returns every line kept so far.
*/
func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.String()
}
