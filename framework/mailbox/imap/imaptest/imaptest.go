/*
|--------------------------------------------------------------------------
| IMAP Test Server
|--------------------------------------------------------------------------
|
| Reading a mailbox is tested against a real server speaking the real
| protocol over a loopback socket, not a stand-in for the client: the
| in-memory server of the IMAP library, behind a fresh certificate the
| test trusts. A server may also speak STARTTLS, or plain text only, to
| test that the client refuses it.
|
*/

package imaptest

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"math/big"
	"net"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-imap/v2/imapserver"
	"github.com/emersion/go-imap/v2/imapserver/imapmemserver"
)

/*
Mode is how the server encrypts connections.
*/
type Mode int

/*
TLS encrypts from the first byte, StartTLS on request, and Plain never.
*/
const (
	TLS Mode = iota
	StartTLS
	Plain
)

/*
Server is a running server. Host and Port are where it listens.
*/
type Server struct {
	Host  string
	Port  int
	mem   *imapmemserver.Server
	users map[string]*imapmemserver.User
	pool  *x509.CertPool
}

/*
New starts a server that stops with the test.
*/
func New(tb testing.TB, mode Mode) *Server {
	tb.Helper()

	cert, pool := certificate(tb)
	config := &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}

	listener, err := (&net.ListenConfig{}).Listen(tb.Context(), "tcp", "127.0.0.1:0")
	check(tb, err)

	options := &imapserver.Options{Logger: quiet{}}

	switch mode {
	case TLS:
		listener = tls.NewListener(listener, config)
	case StartTLS:
		options.TLSConfig = config
	case Plain:
		options.InsecureAuth = true
	}

	mem := imapmemserver.New()
	options.NewSession = func(*imapserver.Conn) (imapserver.Session, *imapserver.GreetingData, error) {
		return mem.NewSession(), nil, nil
	}

	server := imapserver.New(options)

	go func() { _ = server.Serve(listener) }()

	tb.Cleanup(func() { _ = server.Close() })

	address, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		tb.Fatalf("imaptest: listener has no TCP address")
	}

	return &Server{Host: "127.0.0.1", Port: address.Port, mem: mem, users: map[string]*imapmemserver.User{}, pool: pool}
}

/*
Address returns host:port.
*/
func (s *Server) Address() string {
	return net.JoinHostPort(s.Host, strconv.Itoa(s.Port))
}

/*
TLSConfig returns settings that trust the server's certificate.
*/
func (s *Server) TLSConfig() *tls.Config {
	return &tls.Config{RootCAs: s.pool, MinVersion: tls.VersionTLS12}
}

/*
AddUser adds an account holding the named, empty folders.
*/
func (s *Server) AddUser(tb testing.TB, username, password string, folders ...string) {
	tb.Helper()

	user := imapmemserver.NewUser(username, password)
	for _, name := range folders {
		check(tb, user.Create(name, nil))
	}

	s.mem.AddUser(user)
	s.users[username] = user
}

/*
Deliver puts a raw message into a folder, unread, received at the
server's own time.
*/
func (s *Server) Deliver(tb testing.TB, username, folder string, raw []byte) {
	tb.Helper()

	_, err := s.users[username].Append(folder, bytes.NewReader(raw), &imap.AppendOptions{})
	check(tb, err)
}

/*
Renumber deletes the folder and creates it again with its messages, so
the server gives it a new UIDVALIDITY, as a server does after a rebuild.
*/
func (s *Server) Renumber(tb testing.TB, username, password, folder string) {
	tb.Helper()

	held := s.Messages(tb, username, password, folder)
	user := s.users[username]

	check(tb, user.Delete(folder))
	check(tb, user.Create(folder, nil))

	for _, message := range held {
		s.Deliver(tb, username, folder, message.Raw)
	}
}

/*
Message is a message the server holds and its flags.
*/
type Message struct {
	Raw   []byte
	Flags []string
}

/*
Seen says whether the message carries the read flag.
*/
func (m Message) Seen() bool {
	return slices.Contains(m.Flags, string(imap.FlagSeen))
}

/*
Messages reads every message of a folder through a client of its own,
without changing any flag.
*/
func (s *Server) Messages(tb testing.TB, username, password, folder string) []Message {
	tb.Helper()

	client, err := imapclient.DialTLS(s.Address(), &imapclient.Options{TLSConfig: s.TLSConfig()})
	check(tb, err)

	defer func() { _ = client.Close() }()

	check(tb, client.Login(username, password).Wait())

	data, err := client.Select(folder, &imap.SelectOptions{ReadOnly: true}).Wait()
	check(tb, err)

	if data.NumMessages == 0 {
		return nil
	}

	section := &imap.FetchItemBodySection{Peek: true}

	found, err := client.Fetch(imap.SeqSetNum(seq(data.NumMessages)...), &imap.FetchOptions{
		Flags: true, BodySection: []*imap.FetchItemBodySection{section},
	}).Collect()
	check(tb, err)

	out := make([]Message, 0, len(found))
	for _, message := range found {
		flags := make([]string, 0, len(message.Flags))
		for _, flag := range message.Flags {
			flags = append(flags, string(flag))
		}

		out = append(out, Message{Raw: message.FindBodySection(section), Flags: flags})
	}

	return out
}

func seq(count uint32) []uint32 {
	nums := make([]uint32, 0, count)
	for n := range count {
		nums = append(nums, n+1)
	}

	return nums
}

/*
certificate makes a self-signed certificate for the loopback address and
a pool that trusts it.
*/
func certificate(tb testing.TB) (tls.Certificate, *x509.CertPool) {
	tb.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	check(tb, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1), NotBefore: time.Unix(0, 0), NotAfter: time.Unix(1<<32, 0),
		IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1)}, KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, BasicConstraintsValid: true, IsCA: true,
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	check(tb, err)

	parsed, err := x509.ParseCertificate(der)
	check(tb, err)

	pool := x509.NewCertPool()
	pool.AddCert(parsed)

	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}, pool
}

func check(tb testing.TB, err error) {
	tb.Helper()

	if err != nil {
		tb.Fatalf("imaptest: %v", err)
	}
}

/*
quiet drops the server's own log lines, such as a client hanging up.
*/
type quiet struct{}

/*
Printf drops the line.
*/
func (quiet) Printf(string, ...any) {}
