/*
|--------------------------------------------------------------------------
| Message Checks
|--------------------------------------------------------------------------
|
| Every driver checks a message the same way before it reaches the
| provider, so a message refused by one transport is refused by all and
| the refusal is permanent. A line break in a header value would let the
| value add header fields of its own, so none is ever accepted.
|
| A raw message carries its addresses in its own headers. They are read
| from there when the message fields leave them empty, so the envelope
| sender and recipients are known for every message.
|
*/

package mail

import (
	"bytes"
	"fmt"
	"net/mail"
	"net/textproto"
	"slices"
	"strings"
)

/*
reservedHeaders are set from the message fields and the content, never
through Headers.
*/
func reservedHeaders() []string {
	return []string{
		"From", "To", "Cc", "Bcc", "Reply-To", "Subject", "Mime-Version",
		"Content-Type", "Content-Transfer-Encoding", "Content-Disposition",
	}
}

/*
Sender returns the From address, read from the raw message when the
field is empty.
*/
func (m Message) Sender() (Address, error) {
	if m.From.Email != "" || len(m.Raw) == 0 {
		return m.From, nil
	}

	from, err := m.rawAddresses("From")
	if err != nil || len(from) == 0 {
		return Address{}, err
	}

	return from[0], nil
}

/*
Recipients returns every envelope recipient: To, Cc and Bcc. A raw
message with all three empty takes To and Cc from its headers.
*/
func (m Message) Recipients() ([]Address, error) {
	recipients := slices.Concat(m.To, m.Cc, m.Bcc)
	if len(recipients) > 0 || len(m.Raw) == 0 {
		return recipients, nil
	}

	for _, field := range []string{"To", "Cc"} {
		list, err := m.rawAddresses(field)
		if err != nil {
			return nil, err
		}

		recipients = append(recipients, list...)
	}

	return recipients, nil
}

/*
Validate returns a permanent error when the message cannot be sent: no
sender, no recipient, an invalid address, a reserved header in Headers,
or a line break where a header value goes.
*/
func (m Message) Validate() error {
	from, err := m.Sender()
	if err != nil {
		return err
	}

	if from.Email == "" {
		return fmt.Errorf("%w: message has no sender", ErrPermanent)
	}

	recipients, err := m.Recipients()
	if err != nil {
		return err
	}

	if len(recipients) == 0 {
		return fmt.Errorf("%w: message has no recipients", ErrPermanent)
	}

	addresses := append([]Address{from, m.ReplyTo}, recipients...)
	for _, addr := range addresses {
		if err := checkAddress(addr); err != nil {
			return err
		}
	}

	return m.checkHeaders()
}

func (m Message) checkHeaders() error {
	if strings.ContainsAny(m.Subject, "\r\n") {
		return fmt.Errorf("%w: line break in subject", ErrPermanent)
	}

	for name, value := range m.Headers {
		if slices.Contains(reservedHeaders(), textproto.CanonicalMIMEHeaderKey(name)) {
			return fmt.Errorf("%w: header %s is set from the message fields", ErrPermanent, name)
		}

		if name == "" || strings.ContainsAny(name, "\r\n: ") || strings.ContainsAny(value, "\r\n") {
			return fmt.Errorf("%w: invalid header %q", ErrPermanent, name)
		}
	}

	return nil
}

func checkAddress(addr Address) error {
	if addr.Email == "" && addr.Name == "" {
		return nil
	}

	if strings.ContainsAny(addr.Name+addr.Email, "\r\n") {
		return fmt.Errorf("%w: line break in address", ErrPermanent)
	}

	parsed, err := mail.ParseAddress(addr.Email)
	if err != nil || parsed.Address != addr.Email {
		return fmt.Errorf("%w: %w: %q", ErrPermanent, ErrInvalidAddress, addr.Email)
	}

	return nil
}

/*
rawAddresses reads an address header of the raw message; an absent
header has no addresses.
*/
func (m Message) rawAddresses(field string) ([]Address, error) {
	parsed, err := mail.ReadMessage(bytes.NewReader(m.Raw))
	if err != nil {
		return nil, fmt.Errorf("%w: read raw message: %w", ErrPermanent, err)
	}

	if parsed.Header.Get(field) == "" {
		return nil, nil
	}

	list, err := parsed.Header.AddressList(field)
	if err != nil {
		return nil, fmt.Errorf("%w: %s header: %w", ErrPermanent, field, err)
	}

	out := make([]Address, 0, len(list))
	for _, addr := range list {
		out = append(out, Address{Name: addr.Name, Email: addr.Address})
	}

	return out, nil
}
