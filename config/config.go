/*
|--------------------------------------------------------------------------
| Application Configuration
|--------------------------------------------------------------------------
|
| The settings only this product reads, grouped the way operators write
| them in dimmi.yaml or DIMMI_ variables: auth.sessions.idle_lifetime is
| DIMMI_AUTH__SESSIONS__IDLE_LIFETIME. Framework services keep their own
| sections in the framework configuration and are not repeated here.
|
| Several defaults differ between a self-hosted install and the cloud,
| such as whether anyone may sign up. Those are left empty in the tags
| and settled from the edition once everything is loaded, so an operator
| who sets a value always wins.
|
*/

package config

import (
	"errors"
)

/*
ErrInvalid is wrapped by every application setting that cannot work.
*/
var ErrInvalid = errors.New("config: invalid value")

/*
Config is the application's configuration.
*/
type Config struct {
	App  AppConfig  `koanf:"app"`
	API  APIConfig  `koanf:"api"`
	HTTP HTTPConfig `koanf:"http"`
	Auth AuthConfig `koanf:"auth"`

	Workspaces  WorkspacesConfig  `koanf:"workspaces"`
	Invitations InvitationsConfig `koanf:"invitations"`

	Frontend FrontendConfig `koanf:"frontend"`
	Widget   WidgetConfig   `koanf:"widget"`
}

/*
Validate settles the defaults that depend on the edition, then reports
the first value that cannot work. The loader calls it once, right after
the files and environment are read.
*/
func (c *Config) Validate() error {
	c.App.settle()
	c.Auth.settle(c.App.Edition)
	c.Invitations.settle(c.App.Edition)

	return c.Auth.check()
}
