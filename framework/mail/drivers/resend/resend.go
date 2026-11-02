/*
|--------------------------------------------------------------------------
| Resend Mailer
|--------------------------------------------------------------------------
|
| Resend takes a message as JSON fields only, so a message built in
| advance is taken apart into those fields first. Tags travel with the
| message and come back on its delivery events.
|
*/

package resend

import (
	"context"
	"encoding/base64"
	"maps"
	"slices"

	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/mail/internal/provider"
	"github.com/dimmihq/dimmi/framework/mail/mime"
	"github.com/dimmihq/dimmi/framework/manager"
)

/*
Options configure a resend mailer: the API key and the API base URL.
*/
type Options struct {
	APIKey   string `koanf:"api_key"`
	Endpoint string `default:"https://api.resend.com" koanf:"endpoint"`
}

/*
Mailer sends through the Resend API.
*/
type Mailer struct {
	client *httpclient.Client
	opts   Options
}

/*
Factory builds resend mailers for the mail manager.
*/
func Factory(client *httpclient.Client) manager.Factory[mail.Mailer] {
	return func(_ string, section config.Section) (mail.Mailer, error) {
		var opts Options
		if err := section.Decode(&opts); err != nil {
			return nil, err
		}

		return New(client, opts), nil
	}
}

/*
New returns a mailer for the options.
*/
func New(client *httpclient.Client, opts Options) *Mailer {
	return &Mailer{client: client, opts: opts}
}

/*
Send posts the message to the emails endpoint.
*/
func (m *Mailer) Send(ctx context.Context, msg mail.Message) (mail.SendResult, error) {
	msg, err := mime.Decompose(msg)
	if err != nil {
		return mail.SendResult{}, err
	}

	if err := msg.Validate(); err != nil {
		return mail.SendResult{}, err
	}

	resp, err := m.client.New().
		WithToken(m.opts.APIKey).
		Accept("application/json").
		Post(ctx, m.opts.Endpoint+"/emails", body(msg))
	if err := provider.Check("resend", resp, err); err != nil {
		return mail.SendResult{}, err
	}

	var accepted struct{ ID string }
	if err := resp.JSON(&accepted); err != nil {
		return mail.SendResult{}, err
	}

	return mail.SendResult{ProviderMessageID: accepted.ID}, nil
}

func body(msg mail.Message) map[string]any {
	body := map[string]any{
		"from":        msg.From.String(),
		"to":          provider.Formatted(msg.To),
		"cc":          provider.Formatted(msg.Cc),
		"bcc":         provider.Formatted(msg.Bcc),
		"subject":     msg.Subject,
		"text":        msg.Text,
		"html":        msg.HTML,
		"headers":     msg.Headers,
		"tags":        tags(msg.Tags),
		"attachments": attachments(msg.Attachments),
	}

	if msg.ReplyTo.Email != "" {
		body["reply_to"] = []string{msg.ReplyTo.String()}
	}

	return body
}

func tags(in map[string]string) []map[string]string {
	out := make([]map[string]string, 0, len(in))
	for _, name := range slices.Sorted(maps.Keys(in)) {
		out = append(out, map[string]string{"name": name, "value": in[name]})
	}

	return out
}

func attachments(in []mail.Attachment) []map[string]string {
	out := make([]map[string]string, 0, len(in))
	for _, file := range in {
		attachment := map[string]string{
			"filename":     file.Filename,
			"content":      base64.StdEncoding.EncodeToString(file.Content),
			"content_type": file.ContentType,
		}

		if file.Inline {
			attachment["content_id"] = file.ContentID
		}

		out = append(out, attachment)
	}

	return out
}
