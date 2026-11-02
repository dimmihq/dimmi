/*
|--------------------------------------------------------------------------
| Mailgun Mailer
|--------------------------------------------------------------------------
|
| Mailgun accepts a whole MIME document, so the message is built here and
| sent as it is, keeping its Message-ID. Recipients go beside it, which
| is how Bcc recipients receive a copy without being named in it. Tags
| become user variables that come back on delivery events, and tracking
| stays off.
|
*/

package mailgun

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"maps"
	"mime/multipart"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/mail/internal/provider"
	"github.com/dimmihq/dimmi/framework/mail/mime"
	"github.com/dimmihq/dimmi/framework/manager"
)

/*
Options configure a mailgun mailer: the API key, the sending domain, and
the API base URL, https://api.eu.mailgun.net for the EU region.
*/
type Options struct {
	APIKey   string `koanf:"api_key"`
	Domain   string `koanf:"domain"`
	Endpoint string `default:"https://api.mailgun.net" koanf:"endpoint"`
}

/*
Mailer sends through the Mailgun API.
*/
type Mailer struct {
	client  *httpclient.Client
	builder *mime.Builder
	opts    Options
}

/*
Factory builds mailgun mailers for the mail manager.
*/
func Factory(client *httpclient.Client, clk clock.Clock) manager.Factory[mail.Mailer] {
	return func(_ string, section config.Section) (mail.Mailer, error) {
		var opts Options
		if err := section.Decode(&opts); err != nil {
			return nil, err
		}

		return New(client, clk, opts), nil
	}
}

/*
New returns a mailer for the options, dating messages by the clock.
*/
func New(client *httpclient.Client, clk clock.Clock, opts Options) *Mailer {
	return &Mailer{client: client, builder: mime.NewBuilder(clk), opts: opts}
}

/*
Send posts the MIME document to the domain's messages.mime endpoint.
*/
func (m *Mailer) Send(ctx context.Context, msg mail.Message) (mail.SendResult, error) {
	recipients, err := msg.Recipients()
	if err != nil {
		return mail.SendResult{}, err
	}

	raw, err := m.builder.Build(msg)
	if err != nil {
		return mail.SendResult{}, err
	}

	form, contentType, err := encode(raw, recipients, msg.Tags)
	if err != nil {
		return mail.SendResult{}, err
	}

	credentials := base64.StdEncoding.EncodeToString([]byte("api:" + m.opts.APIKey))
	resp, err := m.client.New().
		WithHeader("Authorization", "Basic "+credentials).
		WithHeader("Content-Type", contentType).
		Accept("application/json").
		Post(ctx, m.opts.Endpoint+"/v3/"+m.opts.Domain+"/messages.mime", form)
	if err := provider.Check("mailgun", resp, err); err != nil {
		return mail.SendResult{}, err
	}

	var accepted struct{ ID string }
	if err := resp.JSON(&accepted); err != nil {
		return mail.SendResult{}, err
	}

	return mail.SendResult{ProviderMessageID: strings.Trim(accepted.ID, "<>")}, nil
}

/*
encode writes the multipart form Mailgun expects: recipients, options,
tags as user variables, and the document as the message file.
*/
func encode(raw []byte, recipients []mail.Address, tags map[string]string) ([]byte, string, error) {
	var form bytes.Buffer
	w := multipart.NewWriter(&form)

	fields := [][2]string{
		{"to", strings.Join(provider.Emails(recipients), ",")},
		{"o:tracking", "no"},
		{"o:tracking-clicks", "no"},
		{"o:tracking-opens", "no"},
	}
	for _, name := range slices.Sorted(maps.Keys(tags)) {
		fields = append(fields, [2]string{"v:" + name, tags[name]})
	}

	var errs []error
	for _, field := range fields {
		errs = append(errs, w.WriteField(field[0], field[1]))
	}

	file, err := w.CreateFormFile("message", "message.eml")
	if err == nil {
		_, err = file.Write(raw)
	}

	if err := errors.Join(append(errs, err, w.Close())...); err != nil {
		return nil, "", fmt.Errorf("mailgun form: %w", err)
	}

	return form.Bytes(), w.FormDataContentType(), nil
}
