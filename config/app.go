/*
|--------------------------------------------------------------------------
| Application Settings
|--------------------------------------------------------------------------
|
| The edition decides the defaults that differ between a self-hosted
| install and Dimmi cloud. The region and the API version are reported
| to clients before they sign in, so apps can tell which server they
| talk to and which features it has.
|
*/

package config

/*
Edition is the kind of install the binary runs as.
*/
type Edition string

/*
The editions. A self-hosted install is run by its operator; the cloud
is run by Dimmi for many customers.
*/
const (
	EditionSelfHost Edition = "self_host"
	EditionCloud    Edition = "cloud"
)

/*
Valid reports whether the edition is one the binary knows.
*/
func (e Edition) Valid() bool {
	return e == EditionSelfHost || e == EditionCloud
}

/*
AppConfig describes the install. Region names where the data lives:
"self" on a self-hosted install, a cloud region such as "us" otherwise.
*/
type AppConfig struct {
	Edition Edition `default:"self_host" koanf:"edition"`
	Region  string  `koanf:"region"`
}

/*
APIConfig holds the version of the public API this server speaks.
*/
type APIConfig struct {
	Version string `default:"2026-11-01" koanf:"version"`
}

/*
HTTPConfig lists TrustedOrigins: origins besides the application's own
whose browsers may send unsafe requests with the session cookie, such as
a dashboard served from another host or a local development server.
*/
type HTTPConfig struct {
	TrustedOrigins []string `koanf:"trusted_origins"`
}

/*
settle fills the region a self-hosted install reports when none is set.
*/
func (c *AppConfig) settle() {
	if c.Region == "" && c.Edition == EditionSelfHost {
		c.Region = "self"
	}
}
