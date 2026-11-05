/*
|--------------------------------------------------------------------------
| On-Demand Certificates
|--------------------------------------------------------------------------
|
| The application terminates TLS itself and obtains a certificate from an
| ACME certificate authority the first time a hostname is visited, or
| ahead of time when the hostname is ensured. The authorizer is asked
| before every order, so a hostname nobody verified never costs one of
| the account's limited orders.
|
| Certificates, keys and the ACME account live in PostgreSQL, shared by
| every server, and orders run under a database lock, so a fleet behaves
| like one server towards the authority. Choosing this driver accepts the
| authority's subscriber agreement on the operator's behalf.
|
|   driver: certmagic
|   email: ops@example.com
|   ca: https://acme-v02.api.letsencrypt.org/directory
|
*/

package certmagic

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"slices"

	"github.com/caddyserver/certmagic"
	"go.uber.org/zap"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/customdomain"
	"github.com/dimmihq/dimmi/framework/customdomain/storage"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/manager"
)

/*
ErrEmailRequired is returned for options without an email. Without one,
the ACME library would stop and ask for it on the terminal.
*/
var ErrEmailRequired = errors.New("certmagic: email is required")

/*
Options configure the driver. Email is the address of the ACME account
and is required. CA is the directory URL of the authority, Let's
Encrypt by default. HTTPPort and TLSPort are the ports the server's own
HTTP and HTTPS listeners use, when they are not 80 and 443. The library
then finds them taken and answers challenges through the server's
handlers instead of opening listeners of its own, which an unprivileged
process may not do on the low ports. Logger, set in code, receives the
library's log lines; without it they are dropped.
*/
type Options struct {
	Email    string      `koanf:"email"`
	CA       string      `default:"https://acme-v02.api.letsencrypt.org/directory" koanf:"ca"`
	HTTPPort int         `koanf:"http_port"`
	TLSPort  int         `koanf:"tls_port"`
	Logger   *zap.Logger `koanf:"-"`
}

/*
Driver obtains and serves certificates for authorized hostnames.
*/
type Driver struct {
	authorize customdomain.Authorizer
	storage   *storage.Storage
	cache     *certmagic.Cache
	magic     *certmagic.Config
	issuer    *certmagic.ACMEIssuer
}

var _ customdomain.TLS = (*Driver)(nil)

/*
New returns a driver keeping its certificates in the database and asking
the authorizer before every order. Stop releases it.
*/
func New(db *database.DB, authorize customdomain.Authorizer, opts Options) (*Driver, error) {
	if opts.Email == "" {
		return nil, ErrEmailRequired
	}

	logger := opts.Logger
	if logger == nil {
		logger = zap.NewNop()
	}

	d := &Driver{authorize: authorize, storage: storage.New(db, clock.System())}

	d.cache = certmagic.NewCache(certmagic.CacheOptions{
		GetConfigForCert: func(certmagic.Certificate) (*certmagic.Config, error) { return d.magic, nil },
		Logger:           logger,
	})
	d.magic = certmagic.New(d.cache, certmagic.Config{
		Storage:  d.storage,
		OnDemand: &certmagic.OnDemandConfig{DecisionFunc: d.decide},
		Logger:   logger,
	})
	d.issuer = certmagic.NewACMEIssuer(d.magic, certmagic.ACMEIssuer{
		CA: opts.CA, Email: opts.Email, Agreed: true, Logger: logger,
		AltHTTPPort: opts.HTTPPort, AltTLSALPNPort: opts.TLSPort,
	})
	d.magic.Issuers = []certmagic.Issuer{d.issuer}

	return d, nil
}

/*
Factory builds the driver for the TLS manager from its options, sending
the library's log lines to the logger.
*/
func Factory(
	db *database.DB, authorize customdomain.Authorizer, logger *zap.Logger,
) manager.Factory[customdomain.TLS] {
	return func(name string, section config.Section) (customdomain.TLS, error) {
		var opts Options
		if err := section.Decode(&opts); err != nil {
			return nil, fmt.Errorf("certmagic tls %s: %w", name, err)
		}

		opts.Logger = logger

		driver, err := New(db, authorize, opts)
		if err != nil {
			return nil, fmt.Errorf("certmagic tls %s: %w", name, err)
		}

		return driver, nil
	}
}

/*
Ensure obtains the hostname's certificate now unless one is stored
already, waiting for the order to finish.
*/
func (d *Driver) Ensure(ctx context.Context, host string) error {
	if err := customdomain.Authorize(ctx, d.authorize, host); err != nil {
		return err
	}

	if err := d.magic.ObtainCertSync(ctx, host); err != nil {
		return fmt.Errorf("obtain certificate for %s: %w", host, err)
	}

	return nil
}

/*
Remove drops the hostname's certificate from memory and deletes it from
storage.
*/
func (d *Driver) Remove(ctx context.Context, host string) error {
	issuer := d.issuer.IssuerKey()
	d.cache.RemoveManaged([]certmagic.SubjectIssuer{{Subject: host, IssuerKey: issuer}})

	if err := d.storage.Delete(ctx, certmagic.StorageKeys.CertsSitePrefix(issuer, host)); err != nil {
		return fmt.Errorf("remove certificate for %s: %w", host, err)
	}

	return nil
}

/*
TLSConfig returns the listener configuration: certificates on demand,
the ACME TLS-ALPN challenge, then HTTP/2 and HTTP/1.1.
*/
func (d *Driver) TLSConfig() *tls.Config {
	cfg := d.magic.TLSConfig()
	cfg.NextProtos = slices.Concat(cfg.NextProtos, []string{"h2", "http/1.1"})
	cfg.MinVersion = tls.VersionTLS12

	return cfg
}

/*
HTTPHandler answers ACME HTTP challenges on the plain HTTP listener and
passes every other request to next.
*/
func (d *Driver) HTTPHandler(next http.Handler) http.Handler {
	return d.issuer.HTTPChallengeHandler(next)
}

/*
Stop ends certificate maintenance in the background.
*/
func (d *Driver) Stop() {
	d.cache.Stop()
}

/*
decide is the on-demand decision: a certificate is ordered during a
handshake only for a hostname the authorizer allows.
*/
func (d *Driver) decide(ctx context.Context, name string) error {
	return customdomain.Authorize(ctx, d.authorize, name)
}
