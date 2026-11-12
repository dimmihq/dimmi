/*
|--------------------------------------------------------------------------
| Jira Fixtures
|--------------------------------------------------------------------------
|
| Jira Cloud's API answers for site acme with issue PAY-12, the webhook
| registration, and the deliveries sent when the issue is done or
| closed as won't do. No test ever reaches Atlassian.
|
*/

package jirafixtures

import "embed"

/*
Files are the recorded answers.
*/
//go:embed *.json
var Files embed.FS
