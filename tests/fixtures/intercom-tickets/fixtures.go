/*
|--------------------------------------------------------------------------
| Intercom Ticket Fixtures
|--------------------------------------------------------------------------
|
| Intercom's REST API answers for a workspace with a refund type, a
| payment tracker and three tickets: their states and types, the
| teammates, the requesters, two pages of ticket search and each ticket
| read whole with its parts. Files are named after the path, slashes as
| underscores. No test ever reaches Intercom.
|
*/

package intercomtickets

import "embed"

/*
Files are the recorded answers.
*/
//go:embed *.json
var Files embed.FS
