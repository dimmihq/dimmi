//go:build !dev

/*
|--------------------------------------------------------------------------
| Release Build Commands
|--------------------------------------------------------------------------
|
| Release builds carry no development commands: generators and route
| linting exist only in builds made with the dev tag.
|
*/

package foundation

import "github.com/dimmihq/dimmi/framework/console"

/*
devCommands returns nothing in release builds.
*/
func devCommands(*kernel) []console.Command {
	return nil
}
