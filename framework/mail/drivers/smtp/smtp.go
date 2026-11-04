/*
|--------------------------------------------------------------------------
| SMTP Mailer
|--------------------------------------------------------------------------
|
| An SMTP mailer hands every message to one configured relay or
| submission server, such as a provider's SMTP endpoint or a company
| relay. It never looks up and connects to a recipient's own mail server:
| deliverability is the relay's job.
|
| The connection must be encrypted, by TLS from the start or by STARTTLS,
| unless the configuration explicitly allows plain text for a local
| server. When a DKIM key is configured, the built document is signed
| before it leaves, since a relay does not sign for us.
|
| Server replies decide the error kind: 5xx refuses the message for good,
| 4xx asks to try later, and 421 or a reply mentioning a rate limit asks
| to slow down. A refused login is temporary: the operator fixes it, the
| message itself was fine.
|
*/

package smtp

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net/textproto"
	"regexp"
	"strings"
	"time"

	gomail "github.com/wneessen/go-mail"
	gosmtp "github.com/wneessen/go-mail/smtp"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/mail/dkim"
	"github.com/dimmihq/dimmi/framework/mail/mime"
	"github.com/dimmihq/dimmi/framework/manager"
)

/*
ErrHostRequired is returned when an smtp mailer names no host.
*/
var ErrHostRequired = errors.New("smtp mailer: host is required")

/*
Defaults for zero fields of Options: the submission port and a timeout
for one whole transaction.
*/
const (
	defaultPort    = 587
	defaultTimeout = 30 * time.Second
)

/*
Options configure an smtp mailer.

TLS starts the connection encrypted, as port 465 expects; otherwise
STARTTLS is required unless AllowInsecure accepts a server that does not
offer it. Username turns authentication on. The DKIM options sign every
message when the private key is set.
*/
type Options struct {
	Host           string        `koanf:"host"`
	Port           int           `default:"587" koanf:"port"`
	Username       string        `koanf:"username"`
	Password       string        `koanf:"password"`
	TLS            bool          `koanf:"tls"`
	AllowInsecure  bool          `koanf:"allow_insecure"`
	HELO           string        `koanf:"helo"`
	Timeout        time.Duration `default:"30s" koanf:"timeout"`
	DKIMDomain     string        `koanf:"dkim_domain"`
	DKIMSelector   string        `koanf:"dkim_selector"`
	DKIMPrivateKey string        `koanf:"dkim_private_key"`
}

/*
Mailer sends through an SMTP server.
*/
type Mailer struct {
	client  *gomail.Client
	builder *mime.Builder
	signer  *dkim.Signer
}

/*
Factory builds smtp mailers for the mail manager.
*/
func Factory(clk clock.Clock) manager.Factory[mail.Mailer] {
	return func(_ string, section config.Section) (mail.Mailer, error) {
		var opts Options
		if err := section.Decode(&opts); err != nil {
			return nil, err
		}

		return New(clk, opts)
	}
}

/*
New returns a mailer for the options, dating messages by the clock.
*/
func New(clk clock.Clock, opts Options) (*Mailer, error) {
	if opts.Host == "" {
		return nil, ErrHostRequired
	}

	opts.Port = cmp.Or(opts.Port, defaultPort)
	opts.Timeout = cmp.Or(opts.Timeout, defaultTimeout)

	client, err := gomail.NewClient(opts.Host, clientOptions(opts)...)
	if err != nil {
		return nil, fmt.Errorf("smtp client: %w", err)
	}

	m := &Mailer{client: client, builder: mime.NewBuilder(clk)}
	if opts.DKIMPrivateKey == "" {
		return m, nil
	}

	m.signer, err = dkim.New(dkim.Options{
		Domain: opts.DKIMDomain, Selector: opts.DKIMSelector, PrivateKey: opts.DKIMPrivateKey,
	})
	if err != nil {
		return nil, err
	}

	return m, nil
}

func clientOptions(opts Options) []gomail.Option {
	policy := gomail.TLSMandatory
	if opts.AllowInsecure {
		policy = gomail.TLSOpportunistic
	}

	options := []gomail.Option{
		gomail.WithPort(opts.Port),
		gomail.WithTimeout(opts.Timeout),
		gomail.WithTLSPolicy(policy),
	}

	if opts.TLS {
		options = append(options, gomail.WithSSL())
	}

	if opts.HELO != "" {
		options = append(options, gomail.WithHELO(opts.HELO))
	}

	if opts.Username != "" {
		options = append(options,
			gomail.WithSMTPAuth(gomail.SMTPAuthAutoDiscover),
			gomail.WithUsername(opts.Username),
			gomail.WithPassword(opts.Password),
		)
	}

	return options
}

/*
sign signs the built message with the message's own key, else the
configured one, else leaves it as it is.
*/
func (m *Mailer) sign(msg mail.Message, raw []byte) ([]byte, error) {
	signer := m.signer

	if key := msg.Signing; key != nil {
		own, err := dkim.New(dkim.Options{Domain: key.Domain, Selector: key.Selector, PrivateKey: key.PrivateKey})
		if err != nil {
			return nil, fmt.Errorf("%w: %w", mail.ErrPermanent, err)
		}

		signer = own
	}

	if signer == nil {
		return raw, nil
	}

	return signer.Sign(raw)
}

/*
Send builds, signs and transmits the message on a fresh connection.
*/
func (m *Mailer) Send(ctx context.Context, msg mail.Message) (mail.SendResult, error) {
	from, err := msg.Sender()
	if err != nil {
		return mail.SendResult{}, err
	}

	recipients, err := msg.Recipients()
	if err != nil {
		return mail.SendResult{}, err
	}

	raw, err := m.builder.Build(msg)
	if err != nil {
		return mail.SendResult{}, err
	}

	if raw, err = m.sign(msg, raw); err != nil {
		return mail.SendResult{}, err
	}

	conn, err := m.client.DialToSMTPClientWithContext(ctx)
	if err != nil {
		return mail.SendResult{}, classify("connect", err)
	}

	defer func() { _ = m.client.CloseWithSMTPClient(conn) }()

	reply, err := transmit(conn, from, recipients, raw)
	if err != nil {
		return mail.SendResult{}, err
	}

	return mail.SendResult{ProviderMessageID: queueID(reply)}, nil
}

/*
transmit runs one mail transaction and returns the server's final reply.
*/
func transmit(conn *gosmtp.Client, from mail.Address, recipients []mail.Address, raw []byte) (string, error) {
	if err := conn.Mail("<" + from.Email + ">"); err != nil {
		return "", classify("mail from", err)
	}

	for _, rcpt := range recipients {
		if err := conn.Rcpt("<" + rcpt.Email + ">"); err != nil {
			return "", classify("rcpt to", err)
		}
	}

	data, err := conn.Data()
	if err != nil {
		return "", classify("data", err)
	}

	if _, err := data.Write(raw); err != nil {
		return "", classify("data", err)
	}

	if err := data.Close(); err != nil {
		return "", classify("data", err)
	}

	closer, ok := data.(*gosmtp.DataCloser)
	if !ok {
		return "", nil
	}

	return closer.ServerResponse(), nil
}

/*
classify maps a server reply or connection failure to a mail error kind.
*/
func classify(step string, err error) error {
	var reply *textproto.Error
	if !errors.As(err, &reply) {
		return fmt.Errorf("smtp %s: %w: %w", step, mail.ErrTemporary, err)
	}

	detail := fmt.Sprintf("smtp %s: %d %s", step, reply.Code, reply.Msg)

	switch {
	case reply.Code == 421 || (reply.Code/100 == 4 && strings.Contains(strings.ToLower(reply.Msg), "rate")):
		return fmt.Errorf("%s: %w", detail, &mail.RateLimitedError{})
	case reply.Code/100 == 5 && reply.Code != 535:
		return fmt.Errorf("%s: %w", detail, mail.ErrPermanent)
	default:
		return fmt.Errorf("%s: %w", detail, mail.ErrTemporary)
	}
}

/*
queueID reads the queue id servers put in the final reply, as in
"2.0.0 Ok: queued as 4ABC123".
*/
func queueID(reply string) string {
	match := regexp.MustCompile(`(?i)queued as\s+<?([^\s>]+)`).FindStringSubmatch(reply)
	if match == nil {
		return ""
	}

	return match[1]
}
