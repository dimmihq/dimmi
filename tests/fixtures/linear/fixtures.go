/*
|--------------------------------------------------------------------------
| Linear Fixtures
|--------------------------------------------------------------------------
|
| Linear's GraphQL API answers for an organization with an Engineering team, its
| workflow states and issue ENG-412, and the webhook deliveries it sends
| when the issue moves or is commented on. No test ever reaches Linear.
|
*/

package linearfixtures

import "embed"

/*
Files are the recorded answers.
*/
//go:embed *.json
var Files embed.FS
