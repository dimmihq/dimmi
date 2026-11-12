//go:build !dev

/*
|--------------------------------------------------------------------------
| Live Engineering Tools
|--------------------------------------------------------------------------
|
| Outside development builds every engineering tool is the real one.
|
*/

package providers

import (
	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/issues"
)

/*
fakeTrackers leaves the real tools in place.
*/
func fakeTrackers(*app.App, *issues.Issues) {}
