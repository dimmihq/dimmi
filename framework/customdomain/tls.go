/*
|--------------------------------------------------------------------------
| TLS for Custom Domains
|--------------------------------------------------------------------------
|
| Every custom hostname needs a certificate, and who obtains it depends on
| how the installation is deployed. The application may terminate TLS
| itself and get certificates on demand (certmagic), a proxy in front may
| do it after asking the application whether a hostname is allowed
| (caddy_ask), or the operator's own proxy handles TLS entirely (none).
|
| Whichever driver runs, the application keeps the one decision that
| matters: the authorizer says which hostnames may have a certificate.
| Certificate authorities limit how many certificates an account may
| order, so nothing is ever issued for a hostname the authorizer refuses.
|
|   custom_domains:
|     default: edge
|     tls:
|       edge:
|         driver: certmagic
|         email: ops@example.com
|
*/

package customdomain

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"

	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/manager"
)

/*
ErrNotAuthorized means the authorizer refused a hostname a certificate.
*/
var ErrNotAuthorized = errors.New("customdomain: hostname not authorized for a certificate")

/*
Authorizer decides whether a hostname may have a certificate. It returns
nil to allow it and an error saying why not otherwise. Hostnames arrive
normalized.
*/
type Authorizer func(ctx context.Context, host string) error

/*
Authorize asks the authorizer about the hostname and wraps a refusal in
ErrNotAuthorized. Drivers call it before any work for a hostname.
*/
func Authorize(ctx context.Context, authorize Authorizer, host string) error {
	if err := authorize(ctx, host); err != nil {
		return fmt.Errorf("%w: %s: %w", ErrNotAuthorized, host, err)
	}

	return nil
}

/*
TLS serves custom hostnames over HTTPS.
*/
type TLS interface {
	/*
		Ensure makes the hostname servable over HTTPS, obtaining its
		certificate if the driver manages certificates. It refuses a
		hostname the authorizer does not allow.
	*/
	Ensure(ctx context.Context, host string) error

	/*
		Remove forgets the hostname and deletes any certificate kept for
		it. Removing a hostname that was never ensured is not an error.
	*/
	Remove(ctx context.Context, host string) error

	/*
		TLSConfig returns the configuration for the HTTPS listener, or nil
		when TLS is terminated outside the application.
	*/
	TLSConfig() *tls.Config

	/*
		HTTPHandler wraps the plain HTTP handler so that certificate
		authority challenges are answered before requests reach next.
	*/
	HTTPHandler(next http.Handler) http.Handler
}

/*
Config names the TLS setups and which one is the default. Each section
names its driver.
*/
type Config struct {
	Default string                    `koanf:"default"`
	TLS     map[string]config.Section `koanf:"tls"`
}

/*
Manager resolves named TLS setups and is itself a TLS that uses the
default one.
*/
type Manager struct {
	setups *manager.Manager[TLS]
}

var _ TLS = (*Manager)(nil)

/*
NewManager returns a manager for the setups in the configuration.
Drivers are registered with Extend.
*/
func NewManager(cfg Config) *Manager {
	return &Manager{setups: manager.New[TLS]("custom domain tls", cfg.TLS, cfg.Default)}
}

/*
Extend registers the factory for a driver name.
*/
func (m *Manager) Extend(driver string, factory manager.Factory[TLS]) {
	m.setups.Extend(driver, factory)
}

/*
Setup returns the named TLS setup.
*/
func (m *Manager) Setup(name string) (TLS, error) {
	return m.setups.Instance(name)
}

/*
Default returns the default TLS setup.
*/
func (m *Manager) Default() (TLS, error) {
	return m.setups.Default()
}

/*
Ensure makes the hostname servable with the default setup.
*/
func (m *Manager) Ensure(ctx context.Context, host string) error {
	setup, err := m.Default()
	if err != nil {
		return err
	}

	return setup.Ensure(ctx, host)
}

/*
Remove forgets the hostname in the default setup.
*/
func (m *Manager) Remove(ctx context.Context, host string) error {
	setup, err := m.Default()
	if err != nil {
		return err
	}

	return setup.Remove(ctx, host)
}

/*
TLSConfig returns the default setup's listener configuration, or nil
when the default setup cannot be built or terminates TLS elsewhere.
*/
func (m *Manager) TLSConfig() *tls.Config {
	setup, err := m.Default()
	if err != nil {
		return nil
	}

	return setup.TLSConfig()
}

/*
HTTPHandler wraps the handler with the default setup's challenge
answers, or returns it unchanged when the default setup cannot be built.
*/
func (m *Manager) HTTPHandler(next http.Handler) http.Handler {
	setup, err := m.Default()
	if err != nil {
		return next
	}

	return setup.HTTPHandler(next)
}
