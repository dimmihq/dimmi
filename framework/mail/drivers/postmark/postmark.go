/*
|--------------------------------------------------------------------------
| Postmark Mailer
|--------------------------------------------------------------------------
|
| Postmark takes a message as JSON fields only, so a message built in
| advance is taken apart into those fields first. Separate message
| streams keep kinds of mail apart on one server. Open and link tracking
| stay off: mail must read as written by a person, with no pixels and no
| rewritten links.
|
*/

package postmark

import (
	"context"
	"encoding/base64"
	"strings"

	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/mail/internal/provider"
	"github.com/dimmihq/dimmi/framework/mail/mime"
	"github.com/dimmihq/dimmi/framework/manager"
)

/*
Options configure a postmark mailer: the server token, the message
stream, and the API base URL.
*/
type Options struct {
	Token         string `koanf:"token"`
	MessageStream string `default:"outbound" koanf:"message_stream"`
	Endpoint      string `default:"https://api.postmarkapp.com" koanf:"endpoint"`
}

/*
Mailer sends through the Postmark API.
*/
type Mailer struct {
	client *httpclient.Client
	opts   Options
}

/*
Factory builds postmark mailers for the mail manager.
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
Send posts the message to the email endpoint.
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
		WithHeader("X-Postmark-Server-Token", m.opts.Token).
		Accept("application/json").
		Post(ctx, m.opts.Endpoint+"/email", m.body(msg))
	if err := provider.Check("postmark", resp, err); err != nil {
		return mail.SendResult{}, err
	}

	var accepted struct{ MessageID string }
	if err := resp.JSON(&accepted); err != nil {
		return mail.SendResult{}, err
	}

	return mail.SendResult{ProviderMessageID: accepted.MessageID}, nil
}

func (m *Mailer) body(msg mail.Message) map[string]any {
	body := map[string]any{
		"From":          msg.From.String(),
		"To":            join(msg.To),
		"Cc":            join(msg.Cc),
		"Bcc":           join(msg.Bcc),
		"Subject":       msg.Subject,
		"TextBody":      msg.Text,
		"HtmlBody":      msg.HTML,
		"MessageStream": m.opts.MessageStream,
		"TrackOpens":    false,
		"TrackLinks":    "None",
		"Metadata":      msg.Tags,
		"Headers":       headers(msg.Headers),
		"Attachments":   attachments(msg.Attachments),
	}

	if msg.ReplyTo.Email != "" {
		body["ReplyTo"] = msg.ReplyTo.String()
	}

	return body
}

func join(list []mail.Address) string {
	return strings.Join(provider.Formatted(list), ", ")
}

func headers(in map[string]string) []map[string]string {
	out := make([]map[string]string, 0, len(in))
	for name, value := range in {
		out = append(out, map[string]string{"Name": name, "Value": value})
	}

	return out
}

func attachments(in []mail.Attachment) []map[string]string {
	out := make([]map[string]string, 0, len(in))
	for _, file := range in {
		attachment := map[string]string{
			"Name":        file.Filename,
			"Content":     base64.StdEncoding.EncodeToString(file.Content),
			"ContentType": file.ContentType,
		}

		if file.Inline {
			attachment["ContentID"] = "cid:" + file.ContentID
		}

		out = append(out, attachment)
	}

	return out
}
