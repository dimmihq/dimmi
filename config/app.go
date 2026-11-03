/*
|--------------------------------------------------------------------------
| Application Settings
|--------------------------------------------------------------------------
|
| The edition, chosen when the binary is built, decides the defaults that
| differ between a self-hosted install and Dimmi cloud. The region and the API version are reported
| to clients before they sign in, so apps can tell which server they
| talk to and which features it has.
|
*/

package config

/*
Edition is the kind of install the binary runs as. It is chosen when the
binary is built, never configured.
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
AppConfig describes the install. Edition is the one the binary was
built as. Region names where the data lives: "self" on a self-hosted
install, a cloud region such as "us" otherwise.
*/
type AppConfig struct {
	Edition Edition `koanf:"-"`
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
settle sets the edition the binary was built as, and fills the region a
self-hosted install reports when none is set.
*/
func (c *AppConfig) settle() {
	c.Edition = BuiltEdition
	if c.Region == "" && c.Edition == EditionSelfHost {
		c.Region = "self"
	}
}
