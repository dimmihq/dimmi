/*
|--------------------------------------------------------------------------
| Cloud Boot Guards
|--------------------------------------------------------------------------
|
| Defaults that make a self-hosted install easy are quietly wrong for
| the cloud: files on a container's own disk vanish with it, a log
| mailer sends nobody anything, a plain or local app.url breaks cookies,
| passkeys and links, and no inbound domain means no forwarding address.
| The cloud binary in production refuses to boot with any of them, and
| says which. The self-hosted edition and every other environment are
| left alone.
|
*/

package providers

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/dimmihq/dimmi/framework/foundation"

	appconfig "github.com/dimmihq/dimmi/config"
)

/*
ErrCloudMisconfigured stops the cloud binary from booting in production
with a setting that would quietly lose data or mail.
*/
var ErrCloudMisconfigured = errors.New("cloud production setting")

/*
checkCloudProduction returns every setting the cloud cannot run with in
production, joined, or nil for any other edition or environment.
*/
func checkCloudProduction(app *appconfig.Config, cfg *foundation.Config) error {
	if app.App.Edition != appconfig.EditionCloud || cfg.App.Env != foundation.EnvProduction {
		return nil
	}

	var problems []error
	add := func(key, reason string) {
		problems = append(problems, fmt.Errorf("%w: %s %s", ErrCloudMisconfigured, key, reason))
	}

	if disk := cfg.Filesystems.Disks[cfg.Filesystems.Default]; disk.String("driver") == "local" {
		add("filesystems.default", "keeps files on the container's own disk, which a deploy throws away")
	}

	if mailer := cfg.Mail.Mailers[cfg.Mail.Default]; mailer.String("driver") == "log" {
		add("mail.default", "uses the log mailer, which sends nothing")
	}

	if !publicHTTPS(cfg.App.URL) {
		add("app.url", "must be a public https URL")
	}

	if app.Channels.Email.InboundDomain == "" {
		add("channels.email.inbound_domain", "is required for forwarding addresses")
	}

	return errors.Join(problems...)
}

/*
publicHTTPS reports whether the URL is https on a host other than the
machine itself.
*/
func publicHTTPS(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme != "https" {
		return false
	}

	host := parsed.Hostname()

	return host != "" && host != "localhost" && !strings.HasSuffix(host, ".localhost") &&
		!strings.HasPrefix(host, "127.") && host != "::1"
}
