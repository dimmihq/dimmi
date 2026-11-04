/*
|--------------------------------------------------------------------------
| Bootstrap
|--------------------------------------------------------------------------
|
| Everything the framework needs to know about the product, in one place:
| its name and environment prefix, its own configuration, migrations and
| translations, the browser app's build, and its providers with the
| routes they declare. The
| binary runs it; tests boot the same options with their own
| environment, so a test application is wired exactly like production.
|
| The product carries its own copy of the time zone database, so a host
| or container image without zone files accepts the same time zones for
| workspaces and members as any other.
|
*/

package bootstrap

import (
	/*
		The embedded time zone database, used whenever the host has none.
	*/
	_ "time/tzdata"

	"github.com/dimmihq/dimmi/framework/foundation"

	"github.com/dimmihq/dimmi/app"
	commands "github.com/dimmihq/dimmi/app/console/commands"
	"github.com/dimmihq/dimmi/app/providers"
	"github.com/dimmihq/dimmi/database/migrations"
	"github.com/dimmihq/dimmi/lang"
	"github.com/dimmihq/dimmi/public"
	"github.com/dimmihq/dimmi/routes"
)

/*
The product's name, which names its configuration file, its
environment prefix and its keys in Valkey, and the release it is built
as.
*/
const (
	Name    = "dimmi"
	Version = "0.1.0-dev"
	Module  = "github.com/dimmihq/dimmi"
)

/*
Main runs the command line and returns the exit code.
*/
func Main() int {
	_, opts := Application()

	return foundation.Main(opts)
}

/*
Application returns the product's application, not yet booted, and the
options that boot it.
*/
func Application() (*app.App, foundation.Options) {
	application := app.New(Version)
	application.Frontend = public.Build()
	application.Widget = public.Widget()

	return application, foundation.Options{
		Name:       Name,
		Version:    Version,
		EnvPrefix:  "DIMMI",
		Config:     application.Config,
		Configure:  application.ConfigureFramework,
		Migrations: migrations.Files(),
		Language:   lang.Files(),
		Providers:  providers.All(application, routes.Register),
		Broadcast:  application.ResolveRealtime,
		Commands:   commands.All(application),
		Module:     Module,
	}
}
