/*
|--------------------------------------------------------------------------
| Framework Configuration
|--------------------------------------------------------------------------
|
| One struct holds the configuration of every framework service, keyed
| the way operators write it in YAML or the environment. The application
| loads its own configuration beside it from the same files. A value
| that cannot work stops the boot with every problem listed at once and
| the effective configuration printed with its secrets masked, so an
| operator fixes a deployment in one pass instead of one error per
| restart.
|
*/

package foundation

import (
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/dimmihq/dimmi/framework/broadcasting"
	"github.com/dimmihq/dimmi/framework/cache"
	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/customdomain"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/dns"
	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/filesystem"
	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/http/idempotency"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/i18n"
	"github.com/dimmihq/dimmi/framework/log"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/mailbox"
	"github.com/dimmihq/dimmi/framework/malware"
	"github.com/dimmihq/dimmi/framework/notifications"
	"github.com/dimmihq/dimmi/framework/pubsub"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/ratelimit"
	"github.com/dimmihq/dimmi/framework/redis"
	"github.com/dimmihq/dimmi/framework/session"
	"github.com/dimmihq/dimmi/framework/socialite"
	"github.com/dimmihq/dimmi/framework/telemetry"
)

/*
ErrInvalidConfig is wrapped by every configuration value that cannot
work.
*/
var ErrInvalidConfig = errors.New("foundation: invalid configuration")

/*
Config is the configuration of every framework service, one section per
service under the key operators write it with.
*/
type Config struct {
	App           AppConfig            `koanf:"app"`
	HTTP          HTTPConfig           `koanf:"http"`
	Runtime       RuntimeConfig        `koanf:"runtime"`
	Log           log.Options          `koanf:"log"`
	Telemetry     telemetry.Config     `koanf:"telemetry"`
	Database      database.Config      `koanf:"database"`
	Migrations    MigrationsConfig     `koanf:"migrations"`
	Redis         redis.Config         `koanf:"redis"`
	Cache         cache.Options        `koanf:"cache"`
	RateLimit     ratelimit.Options    `koanf:"rate_limit"`
	PubSub        pubsub.Options       `koanf:"pubsub"`
	Queue         queue.Config         `koanf:"queue"`
	Broadcasting  broadcasting.Config  `koanf:"broadcasting"`
	Filesystems   filesystem.Config    `koanf:"filesystems"`
	Malware       malware.Config       `koanf:"malware"`
	Mail          mail.Config          `koanf:"mail"`
	Mailbox       mailbox.Config       `koanf:"mailbox"`
	Notifications notifications.Config `koanf:"notifications"`
	Hashing       hashing.Config       `koanf:"hashing"`
	Encryption    encryption.Config    `koanf:"encryption"`
	HTTPClient    httpclient.Config    `koanf:"http_client"`
	DNS           dns.Config           `koanf:"dns"`
	AWS           config.Section       `koanf:"aws"`
	Languages     i18n.DetectorConfig  `koanf:"language_detection"`
	Session       session.Config       `koanf:"session"`
	Socialite     socialite.Config     `koanf:"socialite"`
	CustomDomains customdomain.Config  `koanf:"custom_domains"`
	Idempotency   idempotency.Config   `koanf:"idempotency"`
	Maintenance   MaintenanceConfig    `koanf:"maintenance"`
	Gate          GateConfig           `koanf:"gate"`
}

/*
AppConfig describes the application itself.

Env is local, testing or production. URL is the absolute address the
application is reached at, used for generated links. Key is the app
key, "base64:" followed by 32 bytes; when empty it is read from
DataDir/app.key, and generated there first when KeyAutoGenerate is on.
DataDir is the writable directory for local files. Roles lists what
this process does. Locale is the default locale, used when a request
names none and for messages another locale lacks; Locales lists every
locale the application is served in, and adding a language means adding
its lang directory and listing it here. Release names the build that is
running, such as its commit; the image sets it, and a binary built from
a checkout reads its commit from its build information.
*/
type AppConfig struct {
	Env             Environment `default:"production"            koanf:"env"`
	URL             string      `default:"http://localhost:8080" koanf:"url"`
	Key             string      `koanf:"key"                     secret:"true"`
	KeyAutoGenerate bool        `koanf:"key_auto_generate"`
	DataDir         string      `default:"./storage"             koanf:"data_dir"`
	Roles           Roles       `default:"[\"web\",\"worker\",\"scheduler\"]" koanf:"roles"`
	Locale          string      `default:"en"                    koanf:"locale"`
	Locales         []string    `default:"[\"en\"]"              koanf:"locales"`
	Release         string      `koanf:"release"`
}

/*
HTTPConfig configures the listeners.

Addr is the public listener. TLSAddr, when set, is the HTTPS listener,
served with the default custom domain driver's certificates; Addr then
answers only health, certificate challenges and redirects to HTTPS.
AdminAddr is the admin listener for health and metrics, loopback by
default; empty disables it.
TrustedProxies lists the addresses and ranges whose forwarded client
address is believed. ShutdownTimeout bounds the whole graceful
shutdown.
*/
type HTTPConfig struct {
	Addr              string        `default:":8080"          koanf:"addr"`
	TLSAddr           string        `koanf:"tls_addr"`
	AdminAddr         string        `default:"127.0.0.1:9090" koanf:"admin_addr"`
	TrustedProxies    []string      `koanf:"trusted_proxies"`
	ReadHeaderTimeout time.Duration `default:"10s"            koanf:"read_header_timeout"`
	IdleTimeout       time.Duration `default:"120s"           koanf:"idle_timeout"`
	ShutdownTimeout   time.Duration `default:"25s"            koanf:"shutdown_timeout"`
}

/*
GateConfig puts the whole application behind one shared password, for a
public host only invited people should use. Password is what people
type; CIKey is what automated checks send in the X-Gate-Key header. An
enabled gate needs at least one of them.
*/
type GateConfig struct {
	Enabled  bool   `koanf:"enabled"`
	Password string `koanf:"password" secret:"true"`
	CIKey    string `koanf:"ci_key"   secret:"true"`
}

/*
ConfigError stops the boot when the configuration cannot work. Err holds
every problem found, and Redacted the effective configuration with its
secrets masked, for printing beside the error.
*/
type ConfigError struct {
	Err      error
	Redacted map[string]any
}

/*
Error lists the problems.
*/
func (e *ConfigError) Error() string {
	return e.Err.Error()
}

/*
Unwrap returns the problems.
*/
func (e *ConfigError) Unwrap() error {
	return e.Err
}

/*
check returns every value that cannot work, joined.
*/
func (c *Config) check() error {
	problems := []struct {
		found bool
		key   string
		err   error
	}{
		{!c.App.Env.Valid(), "app.env", fmt.Errorf("%w: %q", ErrInvalidEnvironment, c.App.Env)},
		{c.App.DataDir == "", "app.data_dir", errRequired},
		{c.Database.URL == "", "database.url", errRequired},
		{!c.Database.PoolMode.Valid(), "database.pool_mode", database.ErrInvalidPoolMode},
		{c.Redis.URL == "", "redis.url", errRequired},
		{c.HTTP.ShutdownTimeout <= 0, "http.shutdown_timeout", errors.New("must be positive")},
		{c.Gate.Enabled && c.Gate.Password == "" && c.Gate.CIKey == "", "gate.password", errRequired},
	}

	errs := []error{c.App.Roles.validate(), checkURL(c.App.URL), checkMemoryLimit(c.Runtime.MemoryLimit)}
	for _, p := range problems {
		if p.found {
			errs = append(errs, invalid(p.key, p.err))
		}
	}

	return errors.Join(errs...)
}

/*
errRequired is the problem with a value left empty.
*/
var errRequired = errors.New("is required")

/*
invalid names the key whose value has the problem.
*/
func invalid(key string, problem error) error {
	return fmt.Errorf("%w: %s %w", ErrInvalidConfig, key, problem)
}

/*
checkURL accepts an absolute http or https address without a query.
*/
func checkURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return invalid("app.url", err)
	}

	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.RawQuery != "" {
		return invalid("app.url", fmt.Errorf("%q must be an absolute http or https url", raw))
	}

	return nil
}
