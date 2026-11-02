/*
|--------------------------------------------------------------------------
| MIME Decomposition
|--------------------------------------------------------------------------
|
| Some providers accept only JSON fields, never a MIME document. A
| message built in advance is taken apart into those fields here, so the
| provider still sends what the caller built: the same addresses,
| subject, bodies, attachments and threading headers.
|
| Header fields the provider writes itself, such as Date or MIME
| structure, are left out. Addresses already set on the message win
| over the document's headers.
|
*/

package mime

import (
	"bytes"
	"fmt"
	"slices"
	"strings"

	"github.com/jhillyerd/enmime/v2"

	"github.com/dimmihq/dimmi/framework/mail"
)

/*
providerHeaders are written by the provider from the fields or on its
own, never copied as extra headers.
*/
func providerHeaders() []string {
	return []string{
		"From", "To", "Cc", "Bcc", "Reply-To", "Subject", "Date", "Sender",
		"Mime-Version", "Content-Type", "Content-Transfer-Encoding",
		"Content-Disposition", "Received", "Return-Path", "Dkim-Signature",
	}
}

/*
Decompose returns the message with its fields filled from its raw
document and Raw cleared. A message without a raw document is returned
as it is.
*/
func Decompose(msg mail.Message) (mail.Message, error) {
	if len(msg.Raw) == 0 {
		return msg, nil
	}

	env, err := enmime.ReadEnvelope(bytes.NewReader(msg.Raw))
	if err != nil {
		return msg, fmt.Errorf("%w: read raw message: %w", mail.ErrPermanent, err)
	}

	out := msg
	out.Raw = nil
	out.Subject = env.GetHeader("Subject")
	out.Text, out.HTML = env.Text, env.HTML
	out.Headers = headers(env)
	out.Attachments = attachments(env)

	if out.From.Email == "" {
		out.From = first(list(env, "From"))
	}

	if out.ReplyTo.Email == "" {
		out.ReplyTo = first(list(env, "Reply-To"))
	}

	if len(msg.To)+len(msg.Cc)+len(msg.Bcc) == 0 {
		out.To, out.Cc = list(env, "To"), list(env, "Cc")
	}

	return out, nil
}

func headers(env *enmime.Envelope) map[string]string {
	out := map[string]string{}
	for _, name := range env.GetHeaderKeys() {
		if slices.Contains(providerHeaders(), name) {
			continue
		}

		out[name] = strings.Join(env.GetHeaderValues(name), ", ")
	}

	return out
}

func attachments(env *enmime.Envelope) []mail.Attachment {
	var out []mail.Attachment
	for _, part := range env.Attachments {
		out = append(out, attachment(part, false))
	}

	for _, part := range slices.Concat(env.Inlines, env.OtherParts) {
		out = append(out, attachment(part, part.ContentID != ""))
	}

	return out
}

func attachment(part *enmime.Part, inline bool) mail.Attachment {
	return mail.Attachment{
		Filename:    part.FileName,
		ContentType: part.ContentType,
		Content:     part.Content,
		Inline:      inline,
		ContentID:   part.ContentID,
	}
}

func list(env *enmime.Envelope, field string) []mail.Address {
	parsed, err := env.AddressList(field)
	if err != nil {
		return nil
	}

	out := make([]mail.Address, 0, len(parsed))
	for _, a := range parsed {
		out = append(out, mail.Address{Name: a.Name, Email: a.Address})
	}

	return out
}

func first(list []mail.Address) mail.Address {
	if len(list) == 0 {
		return mail.Address{}
	}

	return list[0]
}
