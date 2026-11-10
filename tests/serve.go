/*
|--------------------------------------------------------------------------
| Serving The Test Application
|--------------------------------------------------------------------------
|
| Some clients speak a protocol over real HTTP, such as an MCP client, and
| need an address rather than a handler. The test application is served
| on a local port for the length of the test.
|
*/

package tests

import "github.com/dimmihq/dimmi/framework/testkit"

/*
URL serves the test application on a local port until the test ends and
returns its address.
*/
func (tc *TestCase) URL() string {
	tc.T.Helper()

	return testkit.Serve(tc.T, tc.handler)
}
