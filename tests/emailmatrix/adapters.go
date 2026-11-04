//go:build emailmatrix

/*
|--------------------------------------------------------------------------
| Provider Adapter Runs
|--------------------------------------------------------------------------
|
| Provider documentation does not always match what a provider sends. A run
| mails a real address on the provider's own domain and checks that mail came
| through its webhook, with the signature accepted and the sender's verdict kept.
| With recording on, the delivered payload is saved as a fixture for the driver,
| after every address, network location and secret in it has been replaced.
|
*/

package emailmatrix

import (
	"context"
	"fmt"
	"regexp"

	"github.com/dimmihq/dimmi/framework/filesystem"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/support/str"
)

/*
adapter is one inbound path. Subdomain is empty for an adapter that connects
to a mailbox instead of receiving at a domain of its own.
*/
type adapter struct {
	Name      string
	Subdomain string
}

func adapters() []adapter {
	return []adapter{
		{Name: "postmark", Subdomain: "pm"},
		{Name: "mailgun", Subdomain: "mg"},
		{Name: "sendgrid", Subdomain: "sg"},
		{Name: "resend", Subdomain: "rs"},
		{Name: "cloudflare", Subdomain: "cf"},
		{Name: "imap"},
		{Name: "oauth"},
	}
}

func adapterByName(name string) adapter {
	for _, candidate := range adapters() {
		if candidate.Name == name {
			return candidate
		}
	}

	return adapter{}
}

/*
payloadSource returns the provider payload behind a delivered mail.
*/
type payloadSource interface {
	Payload(ctx context.Context, adapterName, marker string) ([]byte, error)
}

/*
unavailablePayloads stands in until the application exposes delivered payloads
to its operators, and blocks recording.
*/
type unavailablePayloads struct{}

/*
Payload refuses, since reading delivered payloads is not built.
*/
func (unavailablePayloads) Payload(context.Context, string, string) ([]byte, error) {
	return nil, fmt.Errorf("%w: reading delivered payloads is not built", errBlocked)
}

/*
adapterRun is one provider's run. A nil Disk leaves recording off.
*/
type adapterRun struct {
	Adapter  adapter
	Target   adapterTarget
	From     mailbox
	Payloads payloadSource
	Disk     filesystem.Filesystem
}

func (h *harness) runAdapter(ctx context.Context, run adapterRun) report {
	if run.Adapter.Subdomain == "" {
		err := fmt.Errorf("%w: the %s driver is not built", errBlocked, run.Adapter.Name)

		return report{Results: []result{resultOf(run.Adapter.Name, "connected mailbox round trip", err)}}
	}

	marker := "adapter-" + str.Random(12)
	started := h.Clock.Now()
	err := h.adapterDelivery(ctx, run, marker)
	entry := resultOf(run.Adapter.Name, "webhook delivers signed mail with verdicts", err)
	entry.Seconds = h.Clock.Since(started).Seconds()
	results := []result{entry}

	if run.Disk != nil {
		results = append(results, h.record(ctx, run, marker, entry.Status))
	}

	return report{Results: results}
}

func (h *harness) adapterDelivery(ctx context.Context, run adapterRun, marker string) error {
	err := run.From.Out.Send(ctx, mail.Message{
		From:    mail.Address{Email: run.From.Address},
		To:      []mail.Address{{Email: "x@" + run.Target.Domain}},
		Subject: "Adapter check " + marker,
		Text:    "Adapter check " + marker,
	})
	if err != nil {
		return fmt.Errorf("send to %s: %w", run.Target.Domain, err)
	}

	scoped := *h
	scoped.AddressID = run.Target.AddressID

	arrived, err := scoped.waitForInbound(ctx, marker)
	if err != nil {
		return err
	}

	if arrived.AuthResult == "" {
		return fmt.Errorf("mail %s arrived without a verdict", marker)
	}

	return nil
}

func (h *harness) record(ctx context.Context, run adapterRun, marker, delivery string) result {
	const scenario = "payload recorded"

	if delivery != statusPass {
		return result{Host: run.Adapter.Name, Scenario: scenario, Status: statusBlocked, Detail: "needs: delivery"}
	}

	return resultOf(run.Adapter.Name, scenario, savePayload(ctx, run, marker))
}

func savePayload(ctx context.Context, run adapterRun, marker string) error {
	payload, err := run.Payloads.Payload(ctx, run.Adapter.Name, marker)
	if err != nil {
		return fmt.Errorf("read payload: %w", err)
	}

	path := "framework/mailbox/drivers/" + run.Adapter.Name + "/testdata/staging-inbound.json"
	if err := run.Disk.Put(ctx, path, sanitize(payload), filesystem.PutOptions{}); err != nil {
		return fmt.Errorf("save payload: %w", err)
	}

	return nil
}

/*
sanitize replaces what must not reach a public repository: addresses, network
locations, and the value of any field named like a secret. The shape of the
payload, which is what the fixture is for, stays.
*/
func sanitize(payload []byte) []byte {
	secrets := regexp.MustCompile(`(?i)("[^"]*(?:signature|token|secret|password|api_?key)[^"]*"\s*:\s*)"[^"]*"`)
	emails := regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
	addresses := regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)

	cleaned := secrets.ReplaceAll(payload, []byte(`$1"redacted"`))
	cleaned = emails.ReplaceAll(cleaned, []byte("person@example.test"))

	return addresses.ReplaceAll(cleaned, []byte("192.0.2.1"))
}
