/*
|--------------------------------------------------------------------------
| Test Inbound Controller
|--------------------------------------------------------------------------
|
| POST /webhooks/test/inbound takes a raw email, exactly as a receiving
| provider would hand it over, and feeds it through the same pipeline as
| real mail. It exists for end-to-end runs and local development, where
| no provider receives mail for the install.
|
| The route answers only when the install enables the test source, and
| only to a caller holding its shared secret; otherwise it is not found,
| so it reveals nothing. The envelope recipients come from the to
| parameters, or from the message's own recipients; the provider's
| verdicts, which a real provider computes, come from parameters too.
|
*/

package webhooks

import (
	"bytes"
	"cmp"
	"context"
	"io"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/hashing"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/mailbox"
	"github.com/dimmihq/dimmi/framework/mailbox/parse"

	"github.com/dimmihq/dimmi/app/actions/email"
	"github.com/dimmihq/dimmi/app/exceptions"
	appconfig "github.com/dimmihq/dimmi/config"
)

/*
TestInboundController receives raw emails for the test source.
*/
type TestInboundController struct {
	Receive *email.ReceiveInboundEmail
	Config  appconfig.EmailConfig
	Clock   clock.Clock
	Errors  fhttp.Errors
}

/*
Store receives one raw email and answers with how many workspaces it
reached.
*/
func (c *TestInboundController) Store(w fhttp.ResponseWriter, r *fhttp.Request) {
	if !c.Config.SourceEnabled(appconfig.InboundSourceTest) {
		c.Errors.Render(w, r, exceptions.ErrInboundDriverDisabled)

		return
	}

	given := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !hashing.Equal([]byte(given), []byte(c.Config.Inbound.TestSecret)) {
		c.Errors.Render(w, r, exceptions.ErrInboundUnauthorized)

		return
	}

	raw, err := io.ReadAll(io.LimitReader(r.Body, c.Config.Inbound.MaxMessageBytes+1))
	if err != nil || int64(len(raw)) > c.Config.Inbound.MaxMessageBytes || len(raw) == 0 {
		c.Errors.Render(w, r, fhttp.StatusError(fhttp.StatusRequestEntityTooLarge))

		return
	}

	received, err := c.Receive.Handle(r.Context(), c.envelope(r, raw))
	if err != nil {
		c.Errors.Render(w, r, err)

		return
	}

	fhttp.WriteJSON(w, fhttp.StatusAccepted, map[string]any{
		"data": map[string]any{"object": "inbound_delivery", "workspaces": len(received)},
	})
}

/*
envelope describes the raw email as a provider would.
*/
func (c *TestInboundController) envelope(r *fhttp.Request, raw []byte) mailbox.Envelope {
	query := r.URL.Query()
	parsed, err := parse.Message(bytes.NewReader(raw), parse.Limits{})

	recipients := query["to"]
	from := query.Get("from")

	if err == nil && len(recipients) == 0 {
		for _, a := range slices.Concat(parsed.To, parsed.Cc) {
			recipients = append(recipients, a.Email)
		}

		if delivered := parsed.Header.Get("Delivered-To"); delivered != "" {
			recipients = append(recipients, delivered)
		}
	}

	if err == nil && from == "" {
		from = parsed.From.Email
	}

	return mailbox.Envelope{
		Source:            appconfig.InboundSourceTest,
		ProviderMessageID: cmp.Or(query.Get("id"), hashing.SHA256Hex(raw)),
		EnvelopeFrom:      from,
		Recipients:        recipients,
		SizeBytes:         int64(len(raw)),
		ReceivedAt:        c.Clock.Now(),
		Verdicts: mailbox.Verdicts{
			SPF: verdict(query.Get("spf")), DKIM: verdict(query.Get("dkim")), DMARC: verdict(query.Get("dmarc")),
			Spam: verdict(query.Get("spam")), Virus: verdict(query.Get("virus")),
			DMARCPolicy: query.Get("dmarc_policy"),
		},
		OpenRaw: func(context.Context) (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(raw)), nil
		},
	}
}

/*
verdict reads a verdict parameter; anything unknown is none given.
*/
func verdict(value string) mailbox.Verdict {
	switch v := mailbox.Verdict(strings.ToLower(value)); v {
	case mailbox.VerdictPass, mailbox.VerdictFail, mailbox.VerdictNone, mailbox.VerdictUnknown:
		return v
	}

	return ""
}
