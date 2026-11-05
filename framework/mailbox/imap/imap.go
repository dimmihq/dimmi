/*
|--------------------------------------------------------------------------
| IMAP Mailboxes
|--------------------------------------------------------------------------
|
| A mailbox reached with its owner's app password. The password reads the
| mailbox, so the client must never cost the owner anything: folders are
| opened read-only, bodies are fetched with PEEK so no message turns read,
| nothing is moved or deleted, and the only write is a copy of mail the
| application sent, filed into Sent.
|
| The connection is always encrypted, implicit TLS or STARTTLS, and the
| host is dialed through the guarded dialer, so a mailbox setting cannot
| reach into the private network. One session is one connection, opened
| for a sync and closed after it.
|
*/

package imap

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/httpclient/safedial"
)

/*
Errors a session reports while opening.

ErrAuthFailed means the server refused the username or password.
ErrTLSRequired means the server offered no encryption on that port.
ErrHostBlocked means the host is on a network the install may not reach.
ErrUnreachable means no connection could be made or it broke.
ErrNoSentFolder means the mailbox has no folder to file sent mail into.
ErrInvalidOptions means the client lacks its dialer, clock or received
folder.
*/
var (
	ErrAuthFailed     = errors.New("imap: authentication failed")
	ErrTLSRequired    = errors.New("imap: encryption required")
	ErrHostBlocked    = errors.New("imap: host not allowed")
	ErrUnreachable    = errors.New("imap: server unreachable")
	ErrNoSentFolder   = errors.New("imap: no sent folder")
	ErrInvalidOptions = errors.New("imap: dialer, clock and received folder are required")
)

/*
Security is how the connection is encrypted.
*/
type Security string

/*
The two ways in: TLS from the first byte, usually on port 993, or STARTTLS
on port 143. Plain connections are never made.
*/
const (
	SecurityTLS      Security = "tls"
	SecurityStartTLS Security = "starttls"
)

/*
Defaults for a session that options leave unset.
*/
const (
	defaultTimeout   = 60 * time.Second
	defaultBatchSize = 50
)

/*
Credentials reach one mailbox.
*/
type Credentials struct {
	Host     string
	Port     int
	Security Security
	Username string
	Password string
}

/*
Options configure the client. Dialer guards every connection; TLSConfig
is the base of the TLS settings, which only tests change. ReceivedFolder
names the folder of mail that came in. Timeout bounds a whole session,
BatchSize the messages read per call, and MaxMessageBytes the largest
message read whole: a larger one is read as its header only.
*/
type Options struct {
	Dialer          *safedial.Dialer
	TLSConfig       *tls.Config
	Clock           clock.Clock
	ReceivedFolder  string
	Timeout         time.Duration
	BatchSize       int
	MaxMessageBytes int64
}

/*
Client opens sessions on mailboxes.
*/
type Client struct {
	opts Options
}

/*
New returns a client. The dialer, clock and received folder are
required.
*/
func New(opts Options) (*Client, error) {
	if opts.Dialer == nil || opts.Clock == nil || opts.ReceivedFolder == "" {
		return nil, ErrInvalidOptions
	}

	if opts.Timeout <= 0 {
		opts.Timeout = defaultTimeout
	}

	if opts.BatchSize <= 0 {
		opts.BatchSize = defaultBatchSize
	}

	return &Client{opts: opts}, nil
}

/*
Open connects, signs in and finds the folders. The session ends when the
context does, or after the client's timeout, whichever comes first.
*/
func (c *Client) Open(ctx context.Context, creds Credentials) (*Session, error) {
	address := net.JoinHostPort(creds.Host, strconv.Itoa(creds.Port))

	ctx, cancel := context.WithTimeout(ctx, c.opts.Timeout)

	conn, err := c.opts.Dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		cancel()

		return nil, dialError(err, creds.Host)
	}

	stopConn := context.AfterFunc(ctx, func() { _ = conn.Close() })

	client, err := c.handshake(ctx, conn, creds)
	if err != nil {
		cancel()
		_ = conn.Close()

		return nil, err
	}

	stopConn()
	context.AfterFunc(ctx, func() { _ = client.Close() })

	s := &Session{client: client, cancel: cancel, opts: c.opts, received: c.opts.ReceivedFolder}
	if err := s.start(creds); err != nil {
		_ = s.Close()

		return nil, err
	}

	return s, nil
}

func dialError(err error, host string) error {
	if errors.Is(err, safedial.ErrBlocked) {
		return fmt.Errorf("%w: %s", ErrHostBlocked, host)
	}

	return fmt.Errorf("%w: %w", ErrUnreachable, err)
}

/*
handshake encrypts the connection. A server that answers implicit TLS
with plain text, or offers no STARTTLS, is refused.
*/
func (c *Client) handshake(ctx context.Context, conn net.Conn, creds Credentials) (*imapclient.Client, error) {
	config := c.tlsConfig(creds.Host)
	options := &imapclient.Options{TLSConfig: config}

	if creds.Security == SecurityStartTLS {
		client, err := imapclient.NewStartTLS(conn, options)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrTLSRequired, err)
		}

		return client, nil
	}

	secured := tls.Client(conn, config)
	if err := secured.HandshakeContext(ctx); err != nil {
		return nil, tlsError(err)
	}

	return imapclient.New(secured, options), nil
}

func (c *Client) tlsConfig(host string) *tls.Config {
	config := &tls.Config{MinVersion: tls.VersionTLS12}
	if c.opts.TLSConfig != nil {
		config = c.opts.TLSConfig.Clone()
	}

	config.ServerName = host
	config.NextProtos = []string{"imap"}

	return config
}

/*
tlsError tells a server speaking plain text apart from one that cannot be
reached.
*/
func tlsError(err error) error {
	if _, ok := errors.AsType[tls.RecordHeaderError](err); ok {
		return fmt.Errorf("%w: %w", ErrTLSRequired, err)
	}

	return fmt.Errorf("%w: %w", ErrUnreachable, err)
}

/*
commandError reads a server's refusal of the sign-in apart from a broken
connection.
*/
func commandError(err error) error {
	var refused *imap.Error
	if errors.As(err, &refused) && refused.Type == imap.StatusResponseTypeNo {
		return fmt.Errorf("%w: %s", ErrAuthFailed, refused.Text)
	}

	return fmt.Errorf("%w: %w", ErrUnreachable, err)
}
