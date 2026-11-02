/*
|--------------------------------------------------------------------------
| SendGrid Mailer
|--------------------------------------------------------------------------
|
| SendGrid takes a message as JSON fields only, so a message built in
| advance is taken apart into those fields first. Tags travel as custom
| arguments and come back on the delivery events. Click and open
| tracking stay off: mail must read as written by a person, with no
| pixels and no rewritten links.
|
| Optional fields are left out when empty: SendGrid refuses an empty
| list where it expects at least one item. It answers with no body; its
| id for the message comes in the X-Message-Id response header.
|
*/

package sendgrid

import (
	"context"
	"encoding/base64"

	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/mail/internal/provider"
	"github.com/dimmihq/dimmi/framework/mail/mime"
	"github.com/dimmihq/dimmi/framework/manager"
)

/*
Options configure a sendgrid mailer: the API key and the API base URL,
https://api.eu.sendgrid.com for the EU region.
*/
type Options struct {
	APIKey   string `koanf:"api_key"`
	Endpoint string `default:"https://api.sendgrid.com" koanf:"endpoint"`
}

/*
Mailer sends through the SendGrid API.
*/
type Mailer struct {
	client *httpclient.Client
	opts   Options
}

/*
Factory builds sendgrid mailers for the mail manager.
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
Send posts the message to the mail send endpoint.
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
		Post(ctx, m.opts.Endpoint+"/v3/mail/send", body(msg))
	if err := provider.Check("sendgrid", resp, err); err != nil {
		return mail.SendResult{}, err
	}

	return mail.SendResult{ProviderMessageID: resp.Header.Get("X-Message-Id")}, nil
}

func body(msg mail.Message) map[string]any {
	recipients := map[string]any{"to": addresses(msg.To)}
	if len(msg.Cc) > 0 {
		recipients["cc"] = addresses(msg.Cc)
	}

	if len(msg.Bcc) > 0 {
		recipients["bcc"] = addresses(msg.Bcc)
	}

	body := map[string]any{
		"personalizations": []map[string]any{recipients},
		"from":             address(msg.From),
		"subject":          msg.Subject,
		"content":          content(msg),
		"tracking_settings": map[string]any{
			"click_tracking": map[string]bool{"enable": false, "enable_text": false},
			"open_tracking":  map[string]bool{"enable": false},
		},
	}

	if msg.ReplyTo.Email != "" {
		body["reply_to"] = address(msg.ReplyTo)
	}

	if len(msg.Headers) > 0 {
		body["headers"] = msg.Headers
	}

	if len(msg.Tags) > 0 {
		body["custom_args"] = msg.Tags
	}

	if len(msg.Attachments) > 0 {
		body["attachments"] = attachments(msg.Attachments)
	}

	return body
}

/*
content lists the bodies, text first, as SendGrid requires.
*/
func content(msg mail.Message) []map[string]string {
	var out []map[string]string
	if msg.Text != "" {
		out = append(out, map[string]string{"type": "text/plain", "value": msg.Text})
	}

	if msg.HTML != "" {
		out = append(out, map[string]string{"type": "text/html", "value": msg.HTML})
	}

	return out
}

func address(a mail.Address) map[string]string {
	out := map[string]string{"email": a.Email}
	if a.Name != "" {
		out["name"] = a.Name
	}

	return out
}

func addresses(list []mail.Address) []map[string]string {
	out := make([]map[string]string, 0, len(list))
	for _, a := range list {
		out = append(out, address(a))
	}

	return out
}

func attachments(in []mail.Attachment) []map[string]string {
	out := make([]map[string]string, 0, len(in))
	for _, file := range in {
		attachment := map[string]string{
			"filename":    file.Filename,
			"content":     base64.StdEncoding.EncodeToString(file.Content),
			"type":        file.ContentType,
			"disposition": "attachment",
		}

		if file.Inline {
			attachment["disposition"] = "inline"
			attachment["content_id"] = file.ContentID
		}

		out = append(out, attachment)
	}

	return out
}
