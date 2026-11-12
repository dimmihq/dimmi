/*
|--------------------------------------------------------------------------
| Zendesk Ticket Fixtures
|--------------------------------------------------------------------------
|
| Zendesk's API answers for an account with custom statuses, two ticket
| forms and their custom fields, two pages of the incremental ticket
| export with side-loaded users (a problem, an incident of it and a
| solved question), and each ticket's audits. No test ever reaches
| Zendesk.
|
*/

package zendesktickets

import "embed"

/*
Files are the recorded answers.
*/
//go:embed *.json
var Files embed.FS
